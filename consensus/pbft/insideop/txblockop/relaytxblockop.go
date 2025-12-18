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

type RelayTxBlockOp struct {
	c        *chain.Chain
	conn     *network.ConnHandler
	resolver nodetopo.NodeMapper

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewRelayTxBlockOp(c *chain.Chain, conn *network.ConnHandler, rs nodetopo.NodeMapper, cfg config.ConsensusNodeCfg, lp config.LocalParams) *RelayTxBlockOp {
	return &RelayTxBlockOp{c: c, conn: conn, resolver: rs, cfg: cfg, lp: lp}
}

func (r *RelayTxBlockOp) BuildTxBlockProposal(ctx context.Context, txs []transaction.Transaction) (*message.Proposal, error) {
	// if a transaction is a cross-shard tx, modify its RelayOpt
	mTxs, err := r.modifyTxRelayOpt(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("modifyTxRelayOpt failed: %w", err)
	}

	b, err := r.c.GenerateBlock(ctx, r.lp.WalletAddr, mTxs)
	if err != nil {
		return nil, fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	p := message.WrapProposal(b)

	slog.InfoContext(ctx, "block is generated", "shard ID", r.c.GetShardID(), "block height", b.Number, "block create time", b.CreateTime)

	return p, nil
}

// BlockCommitAndDeliver contains:
// 1. apply the proposal to the chain.
// 2.1. send blockInfoMsg to the supervisor.
// 2.2. send relay-txs to leaders of other shards.
func (r *RelayTxBlockOp) BlockCommitAndDeliver(ctx context.Context, isLeader bool, b *block.Block) error {
	// commit block - add block to the blockchain
	if err := r.c.AddBlock(ctx, b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	slog.Info("block is added", "block height", b.Number)

	// if this node is not a leader, skip
	if !isLeader {
		return nil
	}

	// deliver this block info to the supervisor
	innerTxs, r1Txs, r2Txs := r.splitTxs(ctx, b.TxList)

	if err := r.deliverBlockInfo2Supervisor(ctx, innerTxs, r1Txs, r2Txs, *b); err != nil {
		return fmt.Errorf("deliverBlockInfo2Supervisor failed: %w", err)
	}

	if err := r.sendRelayedTxs(ctx, b, r1Txs); err != nil {
		return fmt.Errorf("sendRelayedTxs failed: %w", err)
	}

	return nil
}

// modifyTxRelayOpt sets the RelayOpt of txs.
func (r *RelayTxBlockOp) modifyTxRelayOpt(ctx context.Context, txs []transaction.Transaction) ([]transaction.Transaction, error) {
	accountLocations, err := r.c.GetAccountLocationsInTxs(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	modifiedTxs := make([]transaction.Transaction, 0, len(txs))
	shardID := r.c.GetShardID()

	for _, tx := range txs {
		// if this transaction's relay stage is determined, not modify it
		if tx.RelayStage != transaction.UndeterminedRelayTx {
			modifiedTxs = append(modifiedTxs, tx)
			continue
		}

		senderID := accountLocations[tx.Sender]
		recipientID := accountLocations[tx.Recipient]

		if senderID < 0 || recipientID < 0 {
			return nil, fmt.Errorf("tx sender or recipient does not exist in the accountLocation map")
		}

		if senderID != shardID {
			slog.ErrorContext(ctx, "modify tx relay opt failed, the sender of this tx is not in this shard, and this transaction is not a relay-2 tx", "cur shardID", shardID, "expect shardID", senderID)
			continue
		}

		// this is an inner-shard tx, append it
		if senderID == recipientID {
			modifiedTxs = append(modifiedTxs, tx)
			continue
		}

		// this is a cross-shard tx, modify its RelayOpt
		var thash []byte

		if thash, err = tx.Hash(); err != nil {
			return nil, fmt.Errorf("CalcHash failed: %w", err)
		}

		r1tx := tx
		r1tx.RelayStage = transaction.Relay1Tx
		r1tx.ROriginalHash = thash
		modifiedTxs = append(modifiedTxs, r1tx)
	}

	return modifiedTxs, nil
}

func (r *RelayTxBlockOp) splitTxs(ctx context.Context, txs []transaction.Transaction) ([]transaction.Transaction, []transaction.Transaction, []transaction.Transaction) {
	innerTxs, r1txs, r2txs := make([]transaction.Transaction, 0), make([]transaction.Transaction, 0), make([]transaction.Transaction, 0)

	for _, tx := range txs {
		switch tx.RelayStage {
		case transaction.UndeterminedRelayTx:
			innerTxs = append(innerTxs, tx)
		case transaction.Relay1Tx:
			r1txs = append(r1txs, tx)
		case transaction.Relay2Tx:
			r2txs = append(r2txs, tx)
		default:
			slog.ErrorContext(ctx, "invalid relay tx stage", "relay stage", tx.RelayStage)
		}
	}

	return innerTxs, r1txs, r2txs
}

func (r *RelayTxBlockOp) deliverBlockInfo2Supervisor(ctx context.Context, innerTxs, r1Txs, r2Txs []transaction.Transaction, b block.Block) error {
	rbm := &message.RelayBlockInfoMsg{
		InnerShardTxs:    innerTxs,
		Relay1Txs:        r1Txs,
		Relay2Txs:        r2Txs,
		ShardID:          r.c.GetShardID(),
		Epoch:            r.c.GetEpochID(),
		BlockProposeTime: b.CreateTime,
		BlockCommitTime:  time.Now(),
	}

	w, err := message.WrapMsg(rbm)
	if err != nil {
		return fmt.Errorf("WrapMsg failed: %w", err)
	}

	spv, err := r.resolver.GetSupervisor()
	if err != nil {
		return fmt.Errorf("GetSupervisor failed: %w", err)
	}

	go r.conn.SendMsg2Dest(ctx, spv, w)

	return nil
}

func (r *RelayTxBlockOp) sendRelayedTxs(ctx context.Context, b *block.Block, r1Txs []transaction.Transaction) error {
	accountLocations, err := r.c.GetAccountLocationsInTxs(ctx, b.TxList)
	if err != nil {
		return fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	// for relay1 txs, send relay messages to other shards.
	relayedTxs := make([][]transaction.Transaction, r.cfg.ShardNum)

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

	if err = message.SendWrappedTxs2Shards(ctx, relayedTxs, r.conn, r.resolver); err != nil {
		return fmt.Errorf("SendWrappedTxs2Shards failed: %w", err)
	}

	return nil
}
