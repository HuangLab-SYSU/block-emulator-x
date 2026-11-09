package committee

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator-x/supervisor/txsource"
)

type CLPARelayCommittee struct {
	conn *network.ConnHandler
	r    nodetopo.NodeMapper

	partitionRunner

	txSource    txsource.TxSource
	sl          stopLogic // sl is the logic of stop.
	unsentTxNum int64

	cfg config.SupervisorCfg
}

func NewCLPARelayCommittee(
	conn *network.ConnHandler,
	r nodetopo.NodeMapper,
	cfg config.SupervisorCfg,
) (*CLPARelayCommittee, error) {
	ts, err := txsource.NewTxSource(cfg.TxSourceCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxSource failed: %w", err)
	}

	if n, err := CheckEpochPartitions(cfg.EpochPartitionDir, maxEpochCount(cfg), int(cfg.ShardNum)); err != nil {
		return nil, fmt.Errorf("epoch_partition_dir %q is unusable: %w", cfg.EpochPartitionDir, err)
	} else if n > 0 {
		slog.Info("using externally computed epoch partitions",
			"dir", cfg.EpochPartitionDir, "epochs", n)
	}

	return &CLPARelayCommittee{
		conn: conn,
		r:    r,

		partitionRunner: partitionRunner{
			state:           partition.NewCLPAState(clpaWeightPenalty, clpaMaxIterations, int(cfg.ShardNum)),
			lastRunTime:     time.Now(),
			epochSynced:     false,
			supervisorEpoch: 0,
			shardEpoch:      make([]int64, cfg.ShardNum),
		},

		txSource:    ts,
		sl:          stopLogic{stopThreshold: cfg.ShardNum * stopThresholdPerShard, stopCnt: 0},
		unsentTxNum: cfg.TxNumber,
		cfg:         cfg,
	}, nil
}

func (c *CLPARelayCommittee) SendTxsAndConsensus(ctx context.Context) error {
	// if the repartition procedure between consensus nodes is not over, wait for it and return
	// This function should not be blocked.
	if !c.CheckEpochSyncAndMark() {
		return nil
	}

	// reach epoch duration threshold, run clpa
	if time.Since(c.partitionRunner.lastRunTime).Seconds() > float64(c.cfg.EpochDuration) {
		if err := c.repartition(ctx); err != nil {
			return fmt.Errorf("repartition failed: %w", err)
		}

		return nil
	}

	// read transactions and send them
	if err := c.readTxsAndSend(ctx); err != nil {
		return fmt.Errorf("readTxsAndSend failed: %w", err)
	}

	return nil
}

func (c *CLPARelayCommittee) HandleMsg(_ context.Context, msg *rpcserver.WrappedMsg) error {
	if msg.GetMsgType() != message.RelayBlockInfoMessageType {
		slog.Info("unknown expected msg type", "type", msg.GetMsgType())
		return nil
	}

	var bInfo message.RelayBlockInfoMsg
	if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
		return fmt.Errorf("decode relayBlockInfoMsg: %w", err)
	}

	if bInfo.ShardID >= c.cfg.ShardNum {
		return fmt.Errorf("shard %d out of range", bInfo.ShardID)
	}

	// update the clpa module - shardEpoch
	c.shardEpoch[bInfo.ShardID] = max(c.shardEpoch[bInfo.ShardID], bInfo.Epoch)

	// update the stop module (only after all txs are injected)
	if c.unsentTxNum <= 0 &&
		len(bInfo.InnerShardTxs)+len(bInfo.Relay1Txs)+len(bInfo.Relay2Txs) == 0 {
		c.sl.stopCnt++
	} else if c.unsentTxNum <= 0 {
		c.sl.stopCnt = 0 // reset 0 if there are transactions in a block
	}

	// update the clpa module - graph
	for _, tx := range bInfo.InnerShardTxs {
		c.state.AddEdge(partition.Vertex{Addr: tx.Sender}, partition.Vertex{Addr: tx.Recipient})
	}

	for _, tx := range bInfo.Relay2Txs {
		c.state.AddEdge(partition.Vertex{Addr: tx.Sender}, partition.Vertex{Addr: tx.Recipient})
	}

	return nil
}

func (c *CLPARelayCommittee) ShouldStop() bool {
	return c.sl.stopCnt >= c.sl.stopThreshold
}

func (c *CLPARelayCommittee) repartition(ctx context.Context) error {
	slog.InfoContext(ctx, "repartition start")

	modifiedMap, err := c.nextModifiedMap(ctx)
	if err != nil {
		// Logging alone is not enough: the injection loop keeps injecting, so the whole run
		// finishes without a single repartition while the result files still look normal.
		// Panic so the failure surfaces.
		slog.ErrorContext(ctx, "repartition failed", "err", err)
		panic(fmt.Sprintf("repartition failed: %v", err))
	}

	c.supervisorEpoch++
	cr := &message.CLPARepartitionStartMsg{
		Epoch:       c.supervisorEpoch,
		ModifiedMap: transferMapBytes2Addr(modifiedMap),
	}

	w, err := message.WrapMsg(cr)
	if err != nil {
		return fmt.Errorf("wrapMsg failed: %w", err)
	}

	allLeaders, err := c.r.GetAllLeaders()
	if err != nil {
		return fmt.Errorf("GetAllLeaders failed: %w", err)
	}

	c.conn.GroupBroadcastMessage(ctx, allLeaders, w)

	slog.InfoContext(ctx, "repartition finished", "epoch", c.supervisorEpoch,
		"migrated", len(modifiedMap))

	// Repartition has taken effect, so the table exported here is the partition every shard uses
	// for the next epoch.
	if err := dumpAccountLocations(c.state, c.supervisorEpoch); err != nil {
		slog.ErrorContext(ctx, "account location dump failed",
			"epoch", c.supervisorEpoch, "err", err)
	}

	// set epoch-synced to false
	c.epochSynced = false
	c.sl.stopCnt = 0

	return nil
}

// accountLocDumpEnv names the directory that receives the location tables. Export is off when the
// variable is unset. It is an environment variable rather than a config key so the config struct
// stays unchanged: the dump serves offline analysis only and takes no part in the protocol.
const accountLocDumpEnv = "BEX_ACCOUNT_LOC_DUMP"

// dumpAccountLocations writes the routing table as `epoch,shard,address` CSV, one line per account
// CLPA has placed. Row order is undefined (it comes from a map range); readers should look up by
// address and not rely on line numbers.
func dumpAccountLocations(state *partition.CLPAState, epoch int64) error {
	dir := os.Getenv(accountLocDumpEnv)
	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dump dir: %w", err)
	}

	snap := state.SnapshotLocations()
	path := filepath.Join(dir, fmt.Sprintf("account_location_epoch_%04d.csv", epoch))

	fh, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	defer func() { _ = fh.Close() }()

	w := bufio.NewWriterSize(fh, 1<<20)
	if _, err := w.WriteString("epoch,shard,address\n"); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	var hexa [40]byte

	for v, shard := range snap {
		hex.Encode(hexa[:], v.Addr[:])

		if _, err := fmt.Fprintf(w, "%d,%d,%s\n", epoch, shard, hexa[:]); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}

	return nil
}

// maxEpochCount returns the largest number of repartitions this run can trigger, i.e. how many
// epoch windows the total transaction count spans. The transactions injected in one window are
// fixed by the injection speed and the epoch duration and do not shrink when a repartition pauses,
// so this bound is exact; CheckEpochPartitions relies on it to check every partition file at once.
func maxEpochCount(cfg config.SupervisorCfg) int {
	perEpoch := cfg.TxInjectionSpeed * cfg.EpochDuration
	if perEpoch <= 0 || cfg.TxNumber <= 0 {
		return 0
	}

	return int((cfg.TxNumber + perEpoch - 1) / perEpoch)
}

// nextModifiedMap produces the incremental location table the coming repartition will use: either
// the native CLPA result, or a table computed outside this process (both have the same shape, see
// partitionfile.go).
func (c *CLPARelayCommittee) nextModifiedMap(ctx context.Context) (map[[20]byte]int, error) {
	if c.cfg.EpochPartitionDir == "" {
		modifiedMap, _ := c.state.CLPAPartition()
		return modifiedMap, nil
	}

	next := c.supervisorEpoch + 1

	modifiedMap, err := LoadEpochPartition(c.cfg.EpochPartitionDir, int(next), int(c.cfg.ShardNum))
	if err != nil {
		return nil, err
	}

	// The supervisor routes transactions by the sender location held in CLPAState, and the shards
	// migrate accounts by the modified map broadcast below. Giving the partition to the shards
	// alone would desynchronise the two: the supervisor would keep routing by the initial
	// placement and send transactions to shards that no longer own the sender.
	c.state.SetLocations(modifiedMap)

	slog.InfoContext(ctx, "loaded externally computed partition",
		"epoch", next, "accounts", len(modifiedMap))

	return modifiedMap, nil
}

func (c *CLPARelayCommittee) readTxsAndSend(ctx context.Context) error {
	txs, err := c.txSource.ReadTxs(min(c.cfg.TxInjectionSpeed, c.unsentTxNum))
	if err != nil {
		return fmt.Errorf("failed to read txs: %w", err)
	}

	// send transactions
	shardTxs := packShardTxs(txs, c.cfg.ShardNum, c.getTxLocByCLPAState)
	if err = message.SendWrappedTxs2Shards(ctx, shardTxs, c.conn, c.r); err != nil {
		return fmt.Errorf("failed to send txs to shards: %w", err)
	}

	c.unsentTxNum -= int64(len(txs))

	return nil
}

func (c *CLPARelayCommittee) getTxLocByCLPAState(tx transaction.Transaction) int64 {
	return int64(c.state.GetVertexLocation(partition.Vertex{Addr: tx.Sender}))
}
