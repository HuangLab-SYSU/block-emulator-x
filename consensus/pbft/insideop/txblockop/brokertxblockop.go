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

	slog.InfoContext(
		ctx,
		"block is generated",
		"shard ID",
		bto.c.GetShardID(),
		"block height",
		b.Number,
		"epoch",
		bto.c.GetEpochID(),
		"block create time",
		b.CreateTime,
	)

	return p, nil
}

// BlockCommitAndDeliver contains:
// 1. apply the proposal to the chain.
// 2. send blockInfoMsg to the supervisor.
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

	// deliver this block info to the supervisor
	if err := bto.deliverBlockInfo2Supervisor(ctx, *b); err != nil {
		return fmt.Errorf("deliverBlockInfo2Supervisor failed: %w", err)
	}

	return nil
}

func (*BrokerTxBlockOp) splitTxs(
	ctx context.Context,
	txs []transaction.Transaction,
) ([]transaction.Transaction, []transaction.Transaction, []transaction.Transaction) {
	innerTxs, b1Txs, b2Txs := make(
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

	return innerTxs, b1Txs, b2Txs
}

func (bto *BrokerTxBlockOp) deliverBlockInfo2Supervisor(ctx context.Context, b block.Block) error {
	innerTxs, b1Txs, b2Txs := bto.splitTxs(ctx, b.TxList)
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
