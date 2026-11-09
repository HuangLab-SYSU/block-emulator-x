package committee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
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

	// droppedEdges counts the transactions rejected by the epoch filter since the last repartition,
	// and is reported together with every repartition.
	droppedEdges int64

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

	// Accept only the blocks collected in the current epoch. bInfo.Epoch is the shard's own epoch
	// number, which the shard advances after it commits the migration block, the same condition the
	// epoch-sync check uses. A block of the previous epoch still in flight during the migration
	// carries the old number and is rejected here, so it does not leak into the graph that was just
	// cleared. The graph update is the last step of this function, so returning early skips no
	// logic behind it.
	if bInfo.Epoch != c.supervisorEpoch {
		c.droppedEdges += int64(len(bInfo.InnerShardTxs) + len(bInfo.Relay2Txs))
		return nil
	}

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

	modifiedMap, _ := c.state.CLPAPartition()
	c.supervisorEpoch++

	// The graph belongs to the new epoch from here on: the partition that read it has already been
	// computed, so clear the edges that make it up; from now on HandleMsg accepts only blocks
	// carrying supervisorEpoch. No lock is needed against HandleMsg: both are driven by the same
	// tick loop.
	c.state.EraseEdges()

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

	// Report the number of migrating accounts and the number of transactions dropped by the epoch
	// filter: an empty table and a converged CLPA are indistinguishable in the log, and a filter
	// count that stays at 0 means the filter is not taking effect.
	dropped := c.droppedEdges
	c.droppedEdges = 0

	slog.InfoContext(ctx, "repartition finished", "epoch", c.supervisorEpoch,
		"migrated", len(modifiedMap), "filtered_stale_edges", dropped)

	// set epoch-synced to false
	c.epochSynced = false
	c.sl.stopCnt = 0

	return nil
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
