package committee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource"
)

type CLPARelayCommittee struct {
	conn *network.ConnHandler
	r    nodetopo.NodeMapper

	clpaComponent

	txSource    txsource.TxSource
	sl          stopLogic // sl is the logic of stop.
	unsentTxNum int64

	cfg config.SupervisorCfg
}

func NewCLPARelayCommittee(conn *network.ConnHandler, r nodetopo.NodeMapper, cfg config.SupervisorCfg) (*CLPARelayCommittee, error) {
	ts, err := txsource.NewTxSource(cfg.TxSourceCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxSource failed: %w", err)
	}

	return &CLPARelayCommittee{
		conn: conn,
		r:    r,

		clpaComponent: clpaComponent{
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
	if !c.checkEpochSyncAndMark() {
		return nil
	}

	// reach epoch duration threshold, run clpa
	if time.Since(c.clpaComponent.lastRunTime).Seconds() > float64(c.cfg.EpochDuration) {
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
		return fmt.Errorf("unexpected msg type: %s", msg.GetMsgType())
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

	// update the stop module
	if len(bInfo.InnerShardTxs)+len(bInfo.Relay1Txs)+len(bInfo.Relay2Txs) == 0 {
		c.sl.stopCnt++
	} else {
		c.sl.stopCnt = 0 // reset 0 if there are transactions in a block
	}

	// update the clpa module - graph
	for _, tx := range bInfo.InnerShardTxs {
		c.state.AddEdge(partition.Vertex{Addr: tx.Sender.Addr}, partition.Vertex{Addr: tx.Recipient.Addr})
	}

	for _, tx := range bInfo.Relay2Txs {
		c.state.AddEdge(partition.Vertex{Addr: tx.Sender.Addr}, partition.Vertex{Addr: tx.Recipient.Addr})
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
	cr := &message.CLPARepartitionStartMsg{
		Epoch:       c.supervisorEpoch,
		ModifiedMap: transferMapAddr2Account(modifiedMap),
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

	slog.InfoContext(ctx, "repartition finished", "epoch", c.supervisorEpoch)
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
	return int64(c.state.GetVertexLocation(partition.Vertex{Addr: tx.Sender.Addr}))
}
