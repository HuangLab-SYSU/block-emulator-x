package insideop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
)

// StaticShardOp is the ShardInsideOp for the static-sharding consensus.
type StaticShardOp struct {
	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	tbo txblockop.TxBlockOp
	cfg config.ConsensusNodeCfg
}

func NewStaticShardOp(
	chain *chain.Chain,
	txPool txpool.TxPool,
	tbo txblockop.TxBlockOp,
	cfg config.ConsensusNodeCfg,
) *StaticShardOp {
	return &StaticShardOp{
		chain:  chain,
		txPool: txPool,
		tbo:    tbo,
		cfg:    cfg,
	}
}

func (s *StaticShardOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := s.txPool.PackTxs(int(s.cfg.BlockSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	p, err := s.tbo.BuildTxBlockProposal(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("build block proposal in relay mode failed: %w", err)
	}

	return p, nil
}

func (s *StaticShardOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	if err := s.chain.ValidateBlock(ctx, proposal.Block); err != nil {
		return fmt.Errorf("validate block failed: %w", err)
	}

	return nil
}

func (s *StaticShardOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	if err := s.tbo.BlockCommitAndDeliver(ctx, isLeader, proposal.Block); err != nil {
		return fmt.Errorf("block commit failed: %w", err)
	}

	return nil
}
