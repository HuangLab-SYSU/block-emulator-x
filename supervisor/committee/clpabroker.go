package committee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/broker"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator-x/supervisor/txsource"
)

type CLPABrokerCommittee struct {
	conn *network.ConnHandler
	r    nodetopo.NodeMapper

	clpaComponent
	bManager *broker.Manager

	txSource    txsource.TxSource
	sl          stopLogic // sl is the logic of stop.
	unsentTxNum int64

	cfg config.SupervisorCfg
}

func NewCLPABrokerCommittee(
	conn *network.ConnHandler,
	r nodetopo.NodeMapper,
	cfg config.SupervisorCfg,
) (*CLPABrokerCommittee, error) {
	ts, err := txsource.NewTxSource(cfg.TxSourceCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxSource failed: %w", err)
	}

	bs, err := broker.NewBrokerManager(cfg.BrokerModuleCfg)
	if err != nil {
		return nil, fmt.Errorf("NewBrokerManager failed: %w", err)
	}

	return &CLPABrokerCommittee{
		r:    r,
		conn: conn,
		clpaComponent: clpaComponent{
			state:           partition.NewCLPAState(clpaWeightPenalty, clpaMaxIterations, int(cfg.ShardNum)),
			lastRunTime:     time.Now(),
			epochSynced:     false,
			supervisorEpoch: 0,
			shardEpoch:      make([]int64, cfg.ShardNum),
		},
		bManager:    bs,
		txSource:    ts,
		sl:          stopLogic{stopThreshold: cfg.ShardNum * stopThresholdPerShard, stopCnt: 0},
		unsentTxNum: cfg.TxNumber,
		cfg:         cfg,
	}, nil
}

func (c *CLPABrokerCommittee) SendTxsAndConsensus(ctx context.Context) error {
	// if the repartition process between consensus nodes is not over, wait for it and return
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

func (c *CLPABrokerCommittee) HandleMsg(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	switch msg.GetMsgType() {
	case message.BrokerBlockInfoMessageType:
		var bInfo message.BrokerBlockInfoMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
			return fmt.Errorf("decode relayBlockInfoMsg failed: %w", err)
		}

		c.handleBlockInfoMsg(ctx, &bInfo)
	case message.BrokerCLPATxSendAgainMessageType:
		var tsa message.BrokerCLPATxSendAgainMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&tsa); err != nil {
			return fmt.Errorf("decode relayBlockInfoMsg failed: %w", err)
		}

		c.handleTxSendAgainMsg(ctx, &tsa)
	default:
		return fmt.Errorf("unexpected msg type: %s", msg.GetMsgType())
	}

	return nil
}

func (c *CLPABrokerCommittee) ShouldStop() bool {
	return c.sl.stopCnt >= c.sl.stopThreshold
}

func (c *CLPABrokerCommittee) repartition(ctx context.Context) error {
	slog.InfoContext(ctx, "repartition start")

	modifiedMap, _ := c.state.CLPAPartition()
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

	slog.InfoContext(ctx, "repartition finished", "epoch", c.supervisorEpoch)
	// set epoch-synced to false
	c.epochSynced = false
	c.sl.stopCnt = 0

	return nil
}

func (c *CLPABrokerCommittee) readTxsAndSend(ctx context.Context) error {
	txs, err := c.txSource.ReadTxs(min(c.cfg.TxInjectionSpeed, c.unsentTxNum))
	if err != nil {
		return fmt.Errorf("failed to read txs: %w", err)
	}

	innerTxs, crossTxs := c.classifyTxs(txs)
	// create raw transactions according to the cross-shard txs
	if _, err = c.bManager.CreateRawTxsRandomBroker(crossTxs); err != nil {
		slog.ErrorContext(ctx, "create raw tx failed", "err", err)
	}

	// create broker accounts according to the bManager's ready list.
	b1Txs, b2Txs := c.bManager.CreateBrokerTxs()

	slog.InfoContext(ctx, "ready to send broker transactions", "b1tx size", len(b1Txs), "b2tx size", len(b2Txs))

	sendTxs := append(innerTxs, append(b1Txs, b2Txs...)...)

	// send transactions
	shardTxs := packShardTxs(sendTxs, c.cfg.ShardNum, c.getTxLocByCLPAState)
	if err = message.SendWrappedTxs2Shards(ctx, shardTxs, c.conn, c.r); err != nil {
		return fmt.Errorf("failed to send txs to shards: %w", err)
	}

	c.unsentTxNum -= int64(len(txs))

	return nil
}

func (c *CLPABrokerCommittee) classifyTxs(
	txs []transaction.Transaction,
) ([]transaction.Transaction, []transaction.Transaction) {
	innerShardTxs, crossShardTxs := make(
		[]transaction.Transaction,
		0,
		len(txs),
	), make(
		[]transaction.Transaction,
		0,
		len(txs),
	)
	for _, tx := range txs {
		senderAddr, receiverAddr := tx.Sender, tx.Recipient
		senderShard := c.state.GetVertexLocation(partition.Vertex{Addr: senderAddr})

		receiverShard := c.state.GetVertexLocation(partition.Vertex{Addr: receiverAddr})

		if senderShard == receiverShard || c.bManager.IsBroker(senderAddr) || c.bManager.IsBroker(receiverAddr) {
			innerShardTxs = append(innerShardTxs, tx)
		} else {
			crossShardTxs = append(crossShardTxs, tx)
		}
	}

	return innerShardTxs, crossShardTxs
}

func (c *CLPABrokerCommittee) getTxLocByCLPAState(tx transaction.Transaction) int64 {
	// inner-shard tx
	if len(tx.BOriginalHash) == 0 {
		return int64(c.state.GetVertexLocation(partition.Vertex{Addr: tx.Sender}))
	}
	// broker tx
	// broker 1
	if tx.BrokerStage == transaction.Sigma1BrokerStage {
		return int64(c.state.GetVertexLocation(partition.Vertex{Addr: tx.Sender}))
	}
	// broker 2
	return int64(c.state.GetVertexLocation(partition.Vertex{Addr: tx.Recipient}))
}

func (c *CLPABrokerCommittee) handleBlockInfoMsg(ctx context.Context, bInfo *message.BrokerBlockInfoMsg) {
	if bInfo.ShardID >= c.cfg.ShardNum {
		slog.ErrorContext(ctx, "block info msg is out of range", "shardID", bInfo.ShardID)
		return
	}

	// update the clpa module - shardEpoch
	c.shardEpoch[bInfo.ShardID] = max(c.shardEpoch[bInfo.ShardID], bInfo.Epoch)

	// update the stop module
	if len(bInfo.InnerShardTxs)+len(bInfo.Broker1Txs)+len(bInfo.Broker2Txs) == 0 {
		c.sl.stopCnt++
		return
	}

	c.sl.stopCnt = 0 // reset 0 if there are transactions in a block

	// update the clpa module - graph
	for _, tx := range bInfo.InnerShardTxs {
		c.state.AddEdge(partition.Vertex{Addr: tx.Sender}, partition.Vertex{Addr: tx.Recipient})
	}

	for _, tx := range bInfo.Broker2Txs {
		c.state.AddEdge(partition.Vertex{Addr: tx.Sender}, partition.Vertex{Addr: tx.Recipient})
	}

	// operate as a broker, confirm the transactions.
	for _, broker1Tx := range bInfo.Broker1Txs {
		if err := c.bManager.ConfirmBrokerTx(broker1Tx); err != nil {
			slog.ErrorContext(ctx, "broker confirm broker1 tx failed", "err", err)
		}
	}

	for _, broker2Tx := range bInfo.Broker2Txs {
		if err := c.bManager.ConfirmBrokerTx(broker2Tx); err != nil {
			slog.ErrorContext(ctx, "broker confirm broker2 tx failed", "err", err)
		}
	}
}

// handleTxSendAgainMsg creates a raw broker tx with the given tx.
// Because of the account migration, an inner-shard tx may be changed into a cross-shard one.
// This supervisor should re-send this tx as a broker tx.
func (c *CLPABrokerCommittee) handleTxSendAgainMsg(ctx context.Context, tsa *message.BrokerCLPATxSendAgainMsg) {
	if _, err := c.bManager.CreateRawTxsRandomBroker(tsa.Txs); err != nil {
		slog.ErrorContext(ctx, "create raw tx failed", "err", err)
		return
	}

	slog.Info("create raw txs from BrokerCLPATxSendAgainMsg", "tx size", len(tsa.Txs))
}
