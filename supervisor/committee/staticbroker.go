package committee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

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

type StaticBrokerCommittee struct {
	r    nodetopo.NodeMapper  // r give the information of other nodes.
	conn *network.ConnHandler // conn is the p2p-connections among consensus nodes, i.e., network layer.

	bManager *broker.Manager // bManager controls the brokers and their states.

	txSource    txsource.TxSource // txSource brings the txs into the blockchain system.
	sl          stopLogic         // sl is the logic of stop.
	unsentTxNum int64

	cfg config.SupervisorCfg
}

func NewStaticBrokerCommittee(
	conn *network.ConnHandler,
	r nodetopo.NodeMapper,
	cfg config.SupervisorCfg,
) (*StaticBrokerCommittee, error) {
	ts, err := txsource.NewTxSource(cfg.TxSourceCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxSource failed: %w", err)
	}

	bs, err := broker.NewBrokerManager(cfg.BrokerModuleCfg)
	if err != nil {
		return nil, fmt.Errorf("NewBrokerManager failed: %w", err)
	}

	return &StaticBrokerCommittee{
		r:           r,
		conn:        conn,
		bManager:    bs,
		txSource:    ts,
		sl:          stopLogic{stopThreshold: cfg.ShardNum * stopThresholdPerShard, stopCnt: 0},
		unsentTxNum: cfg.TxNumber,
		cfg:         cfg,
	}, nil
}

func (s *StaticBrokerCommittee) SendTxsAndConsensus(ctx context.Context) error {
	if err := s.readTxsAndSend(ctx); err != nil {
		return fmt.Errorf("readTxsAndSend failed: %w", err)
	}

	return nil
}

func (s *StaticBrokerCommittee) HandleMsg(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	if msg.GetMsgType() != message.BrokerBlockInfoMessageType {
		slog.Info("unknown expected msg type", "type", msg.GetMsgType())
		return nil
	}

	var bInfo message.BrokerBlockInfoMsg
	if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
		return fmt.Errorf("decode relayBlockInfoMsg: %w", err)
	}

	// update the stop module
	if len(bInfo.InnerShardTxs)+len(bInfo.Broker1Txs)+len(bInfo.Broker2Txs) == 0 {
		s.sl.stopCnt++
	} else {
		s.sl.stopCnt = 0 // reset 0 if there are transactions in a block
	}

	// operate as a broker, confirm the transactions.
	for _, broker1Tx := range bInfo.Broker1Txs {
		if err := s.bManager.ConfirmBrokerTx(broker1Tx); err != nil {
			slog.ErrorContext(ctx, "broker confirm broker1 tx failed", "err", err)
		}
	}

	for _, broker2Tx := range bInfo.Broker2Txs {
		if err := s.bManager.ConfirmBrokerTx(broker2Tx); err != nil {
			slog.ErrorContext(ctx, "broker confirm broker2 tx failed", "err", err)
		}
	}

	return nil
}

func (s *StaticBrokerCommittee) ShouldStop() bool {
	return s.sl.stopCnt >= s.sl.stopThreshold
}

func (s *StaticBrokerCommittee) readTxsAndSend(ctx context.Context) error {
	txs, err := s.txSource.ReadTxs(min(s.cfg.TxInjectionSpeed, s.unsentTxNum))
	if err != nil {
		return fmt.Errorf("failed to read txs: %w", err)
	}

	innerTxs, crossTxs := s.classifyTxs(txs)
	// create raw transactions according to the cross-shard txs
	if _, err = s.bManager.CreateRawTxsRandomBroker(crossTxs); err != nil {
		slog.ErrorContext(ctx, "create raw tx failed", "err", err)
	}
	// create broker accounts
	b1Txs, b2Txs := s.bManager.CreateBrokerTxs()

	sendTxs := append(innerTxs, append(b1Txs, b2Txs...)...)

	// send transactions
	shardTxs := packShardTxs(sendTxs, s.cfg.ShardNum, s.getTxLoc)
	if err = message.SendWrappedTxs2Shards(ctx, shardTxs, s.conn, s.r); err != nil {
		return fmt.Errorf("failed to send txs to shards: %w", err)
	}

	s.unsentTxNum -= int64(len(txs))

	return nil
}

func (s *StaticBrokerCommittee) classifyTxs(
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
		senderShard := partition.DefaultAccountLoc(senderAddr, s.cfg.ShardNum)

		receiverShard := partition.DefaultAccountLoc(receiverAddr, s.cfg.ShardNum)

		if senderShard == receiverShard || s.bManager.IsBroker(senderAddr) || s.bManager.IsBroker(receiverAddr) {
			innerShardTxs = append(innerShardTxs, tx)
		} else {
			crossShardTxs = append(crossShardTxs, tx)
		}
	}

	return innerShardTxs, crossShardTxs
}

func (s *StaticBrokerCommittee) getTxLoc(tx transaction.Transaction) int64 {
	shardNumber := s.cfg.ShardNum
	// inner-shard tx
	if tx.TxType() == transaction.NormalTxType {
		return partition.DefaultAccountLoc(tx.Sender, shardNumber)
	}
	// broker tx
	// broker 1
	if tx.BrokerStage == transaction.Sigma1BrokerStage {
		return partition.DefaultAccountLoc(tx.Sender, shardNumber)
	}
	// broker 2
	return partition.DefaultAccountLoc(tx.Recipient, shardNumber)
}
