package txblockop

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

// BrokerTxBlockOp is the TxBlockOp which can handle broker transactions.
type BrokerTxBlockOp struct {
	c        *chain.Chain
	conn     *network.ConnHandler
	resolver nodetopo.NodeMapper

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewBrokerTxBlockOp(
	c *chain.Chain,
	conn *network.ConnHandler,
	rs nodetopo.NodeMapper,
	cfg config.ConsensusNodeCfg,
	lp config.LocalParams,
) *BrokerTxBlockOp {
	return &BrokerTxBlockOp{c: c, conn: conn, resolver: rs, cfg: cfg, lp: lp}
}

func (bto *BrokerTxBlockOp) BuildTxBlockProposal(
	ctx context.Context,
	txs []transaction.Transaction,
) (*message.Proposal, error) {
	b, err := bto.c.GenerateBlock(
		ctx,
		bto.lp.WalletAddr,
		block.TxBlockType,
		block.Body{TxList: txs},
		block.MigrationOpt{},
	)
	if err != nil {
		return nil, fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	p := message.WrapProposal(b)

	return p, nil
}

// BlockCommitAndDeliver contains:
// 1. apply the proposal to the chain.
// 2. send blockInfoMsg to the supervisor.
// 3. send relay-txs to leaders of other shards (for fallback mechanism).
func (bto *BrokerTxBlockOp) BlockCommitAndDeliver(ctx context.Context, isLeader bool, b *block.Block) error {
	// commit block - add block to the blockchain
	if err := bto.c.AddBlock(ctx, b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	slog.Info("block is added", "block height", b.Number)

	// if this node is not the leader, skip it
	if !isLeader {
		return nil
	}

	innerTxs, b1Txs, b2Txs, r1Txs := bto.splitTxs(ctx, b.TxList)

	// deliver this block info to the supervisor
	if err := bto.deliverBlockInfo2Supervisor(ctx, innerTxs, b1Txs, b2Txs, *b); err != nil {
		return fmt.Errorf("deliverBlockInfo2Supervisor failed: %w", err)
	}

	// handle relay fallback: send relay2 txs to destination shards
	if len(r1Txs) > 0 {
		if err := bto.sendRelayedTxs(ctx, b, r1Txs); err != nil {
			return fmt.Errorf("sendRelayedTxs failed: %w", err)
		}
	}

	return nil
}

func (bto *BrokerTxBlockOp) splitTxs(
	ctx context.Context,
	txs []transaction.Transaction,
) ([]transaction.Transaction, []transaction.Transaction, []transaction.Transaction, []transaction.Transaction) {
	innerTxs, b1Txs, b2Txs, r1Txs := make(
		[]transaction.Transaction,
		0,
	), make(
		[]transaction.Transaction,
		0,
	), make(
		[]transaction.Transaction,
		0,
	), make(
		[]transaction.Transaction,
		0,
	)

	for _, tx := range txs {
		// if it is a relay fallback tx
		if tx.TxType() == transaction.RelayTxType {
			if tx.RelayStage == transaction.Relay1Tx {
				r1Txs = append(r1Txs, tx)
			} else {
				innerTxs = append(innerTxs, tx)
			}
			continue
		}

		switch tx.BrokerStage {
		case transaction.RawTxBrokerStage:
			innerTxs = append(innerTxs, tx)
		case transaction.Sigma1BrokerStage:
			b1Txs = append(b1Txs, tx)
		case transaction.Sigma2BrokerStage:
			b2Txs = append(b2Txs, tx)
		default:
			slog.ErrorContext(
				ctx,
				"broker-handler split tx error, broker stage invalid",
				"broker stage",
				tx.BrokerStage,
			)
		}
	}

	return innerTxs, b1Txs, b2Txs, r1Txs
}

func (bto *BrokerTxBlockOp) deliverBlockInfo2Supervisor(ctx context.Context, innerTxs, b1Txs, b2Txs []transaction.Transaction, b block.Block) error {
	bbm := &message.BrokerBlockInfoMsg{
		InnerShardTxs:    innerTxs,
		Broker1Txs:       b1Txs,
		Broker2Txs:       b2Txs,
		Epoch:            bto.c.GetEpochID(),
		ShardID:          bto.c.GetShardID(),
		BlockProposeTime: b.CreateTime,
		BlockCommitTime:  time.Now(),
	}

	w, err := message.WrapMsg(bbm)
	if err != nil {
		return fmt.Errorf("WrapMsg failed: %w", err)
	}

	spv, err := bto.resolver.GetSupervisor()
	if err != nil {
		return fmt.Errorf("GetSupervisor failed: %w", err)
	}

	go bto.conn.SendMsg2Dest(ctx, spv, w)

	return nil
}

func (bto *BrokerTxBlockOp) sendRelayedTxs(ctx context.Context, b *block.Block, r1Txs []transaction.Transaction) error {
	accountLocations, err := bto.c.GetAccountLocationsInTxs(ctx, b.TxList)
	if err != nil {
		return fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	// for relay1 txs, send relay messages to other shards.
	relayedTxs := make([][]transaction.Transaction, bto.cfg.ShardNum)

	// split r1Txs into all shards
	for _, tx := range r1Txs {
		// the next destination of relay1 tx should be calculated according to the recipient addr.
		shardID, ok := accountLocations[tx.Recipient]
		if !ok {
			slog.ErrorContext(ctx, "tx.Recipient is not found in accountLocations", "recipient", tx.Recipient)
			continue
		}

		// modify relay transaction's RelayOpt
		updatedRelayedTx := tx
		updatedRelayedTx.RelayStage = transaction.Relay2Tx
		relayedTxs[shardID] = append(relayedTxs[shardID], updatedRelayedTx)
	}

	if err = message.SendWrappedTxs2Shards(ctx, relayedTxs, bto.conn, bto.resolver); err != nil {
		return fmt.Errorf("SendWrappedTxs2Shards failed: %w", err)
	}

	return nil
}
