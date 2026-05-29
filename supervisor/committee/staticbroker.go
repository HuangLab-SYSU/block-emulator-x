package committee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"math/big"
	"math/rand/v2"
	"sync"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/broker"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
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

	brokerBalances    map[account.Address][]*big.Int // brokerBalances records the balances of brokers in each shard.
	pendingDeductions map[account.Address][]*big.Int // pendingDeductions records the deductions of brokers in each shard.
	mu                sync.RWMutex
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

	brokerBalances := make(map[account.Address][]*big.Int)
	pendingDeductions := make(map[account.Address][]*big.Int)
	// initialize the balances of brokers
	for _, bAddr := range bs.GetBrokers() {
		brokerBalances[bAddr] = make([]*big.Int, cfg.ShardNum)
		pendingDeductions[bAddr] = make([]*big.Int, cfg.ShardNum)
		for i := int64(0); i < cfg.ShardNum; i++ {
			bal := new(big.Int)
			bal.SetString(account.BrokerInitBalanceStr, 10)
			brokerBalances[bAddr][i] = bal
			pendingDeductions[bAddr][i] = big.NewInt(0)
		}
	}

	return &StaticBrokerCommittee{
		r:                 r,
		conn:              conn,
		bManager:          bs,
		txSource:          ts,
		sl:                stopLogic{stopThreshold: cfg.ShardNum * stopThresholdPerShard, stopCnt: 0},
		unsentTxNum:       cfg.TxNumber,
		cfg:               cfg,
		brokerBalances:    brokerBalances,
		pendingDeductions: pendingDeductions,
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

	// Only count empty blocks toward stop after all txs are injected.
	// Otherwise startup empty blocks cause the supervisor to exit before injection.
	if s.unsentTxNum <= 0 {
		if len(bInfo.InnerShardTxs)+len(bInfo.Broker1Txs)+len(bInfo.Broker2Txs) == 0 {
			s.sl.stopCnt++
		} else {
			s.sl.stopCnt = 0 // reset 0 if there are transactions in a block
		}
	}

	// operate as a broker, confirm the transactions.
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, broker1Tx := range bInfo.Broker1Txs {
		if err := s.bManager.ConfirmBrokerTx(broker1Tx); err != nil {
			slog.ErrorContext(ctx, "broker confirm broker1 tx failed", "err", err)
		} else {
			// Broker receives money in the sender shard (bInfo.ShardID)
			s.brokerBalances[broker1Tx.Broker][bInfo.ShardID].Add(s.brokerBalances[broker1Tx.Broker][bInfo.ShardID], broker1Tx.Value)
		}
	}

	for _, broker2Tx := range bInfo.Broker2Txs {
		if err := s.bManager.ConfirmBrokerTx(broker2Tx); err != nil {
			slog.ErrorContext(ctx, "broker confirm broker2 tx failed", "err", err)
		} else {
			// Broker pays money in the recipient shard (bInfo.ShardID)
			s.brokerBalances[broker2Tx.Broker][bInfo.ShardID].Sub(s.brokerBalances[broker2Tx.Broker][bInfo.ShardID], broker2Tx.Value)
			// Clear the pending deduction
			pending := s.pendingDeductions[broker2Tx.Broker][bInfo.ShardID]
			pending.Sub(pending, broker2Tx.Value)
			if pending.Sign() < 0 {
				pending.SetInt64(0)
			}
		}
	}

	return nil
}

func (s *StaticBrokerCommittee) ShouldStop() bool {
	return s.sl.stopCnt >= s.sl.stopThreshold && s.bManager.IsFinished()
}

func (s *StaticBrokerCommittee) readTxsAndSend(ctx context.Context) error {
	txs, err := s.txSource.ReadTxs(min(s.cfg.TxInjectionSpeed, s.unsentTxNum))
	if err != nil {
		return fmt.Errorf("failed to read txs: %w", err)
	}

	innerTxs, crossTxs := s.classifyTxs(txs)

	relayTxs := make([]transaction.Transaction, 0)
	brokerRawTxs := make([]transaction.Transaction, 0)

	s.mu.Lock()
	for _, tx := range crossTxs {
		senderShard := partition.DefaultAccountLoc(tx.Sender, s.cfg.ShardNum)
		recipientShard := partition.DefaultAccountLoc(tx.Recipient, s.cfg.ShardNum)
		foundBroker := false

		// try to find a broker with enough balance
		brokers := s.bManager.GetBrokers()
		shuffledBrokers := make([]account.Address, len(brokers))
		copy(shuffledBrokers, brokers)
		rand.Shuffle(len(shuffledBrokers), func(i, j int) {
			shuffledBrokers[i], shuffledBrokers[j] = shuffledBrokers[j], shuffledBrokers[i]
		})

		for _, bAddr := range shuffledBrokers {
			balance := s.brokerBalances[bAddr][recipientShard]
			pending := s.pendingDeductions[bAddr][recipientShard]

			available := new(big.Int).Sub(balance, pending)
			if available.Cmp(tx.Value) >= 0 {
				// found a broker!
				s.pendingDeductions[bAddr][recipientShard].Add(pending, tx.Value)
				rawTx, err := s.bManager.CreateRawTx(tx, bAddr)
				if err != nil {
					slog.ErrorContext(ctx, "create raw tx failed", "err", err)
					continue
				}
				brokerRawTxs = append(brokerRawTxs, *rawTx)
				foundBroker = true
				break
			}
		}

		if !foundBroker {
			// fallback to relay
			slog.Warn("All brokers out of balance! Falling back to Relay mechanism.",
				"from shardID", senderShard,
				"to shardID", recipientShard)

			th, _ := tx.Hash()
			tx.RelayTxOpt = transaction.RelayTxOpt{
				RelayStage:    transaction.Relay1Tx,
				ROriginalHash: th,
			}
			relayTxs = append(relayTxs, tx)
		}
	}
	s.mu.Unlock()

	// create broker accounts
	b1Txs, b2Txs := s.bManager.CreateBrokerTxs()

	sendTxs := append(innerTxs, append(relayTxs, append(b1Txs, b2Txs...)...)...)

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
	txType := tx.TxType()

	// inner-shard tx or relay 1
	if txType == transaction.NormalTxType || (txType == transaction.RelayTxType && tx.RelayStage == transaction.Relay1Tx) {
		return partition.DefaultAccountLoc(tx.Sender, shardNumber)
	}
	// relay 2
	if txType == transaction.RelayTxType && tx.RelayStage == transaction.Relay2Tx {
		return partition.DefaultAccountLoc(tx.Recipient, shardNumber)
	}

	// broker tx
	// broker 1
	if tx.BrokerStage == transaction.Sigma1BrokerStage {
		return partition.DefaultAccountLoc(tx.Sender, shardNumber)
	}
	// broker 2
	return partition.DefaultAccountLoc(tx.Recipient, shardNumber)
}
