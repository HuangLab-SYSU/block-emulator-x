package insideop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/csvwrite"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
)

type StaticShardOp struct {
	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	tbo txblockop.TxBlockOp

	csw *csvwrite.CSVSeqWriter
	cfg config.ConsensusNodeCfg
}

func NewStaticShardOp(chain *chain.Chain, txPool txpool.TxPool, tbo txblockop.TxBlockOp, csw *csvwrite.CSVSeqWriter, cfg config.ConsensusNodeCfg) *StaticShardOp {
	return &StaticShardOp{
		chain:  chain,
		txPool: txPool,
		tbo:    tbo,
		csw:    csw,
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
	if proposal.ProposalType != message.BlockProposalType {
		return fmt.Errorf("invalid proposal type")
	}

	b, err := block.DecodeBlock(proposal.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	if err = s.chain.ValidateBlock(ctx, b); err != nil {
		return fmt.Errorf("validate block failed: %w", err)
	}

	return nil
}

func (s *StaticShardOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	b, err := block.DecodeBlock(proposal.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	switch proposal.ProposalType {
	case message.BlockProposalType:
		if err = s.tbo.BlockCommitAndDeliver(ctx, isLeader, b); err != nil {
			return fmt.Errorf("block commit failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type=%s", proposal.ProposalType)
	}

	if isLeader {
		if err = recordBlock(s.csw, b); err != nil {
			return fmt.Errorf("record block failed: %w", err)
		}
	}

	return nil
}
