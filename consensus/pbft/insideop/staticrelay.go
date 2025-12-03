package insideop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type StaticRelayInsideOp struct {
	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	*txblockop.RelayTxBlockOp

	cfg config.ConsensusNodeCfg
}

func NewStaticRelayInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper, chain *chain.Chain, txPool txpool.TxPool, cfg config.ConsensusNodeCfg, lp config.LocalParams) (*StaticRelayInsideOp, error) {
	rh, err := txblockop.NewRelayTxBlockOp(chain, conn, resolver, cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewRelayTxBlockOp failed: %w", err)
	}

	return &StaticRelayInsideOp{
		chain:          chain,
		txPool:         txPool,
		RelayTxBlockOp: rh,
		cfg:            cfg,
	}, nil
}

func (s *StaticRelayInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := s.txPool.PackTxs(int(s.cfg.BlockSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	p, err := s.BuildTxBlockProposal(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("build block proposal in relay mode failed: %w", err)
	}

	return p, nil
}

func (s *StaticRelayInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
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

func (s *StaticRelayInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	var (
		b   *block.Block
		err error
	)

	switch proposal.ProposalType {
	case message.BlockProposalType:
		b, err = block.DecodeBlock(proposal.Payload)
		if err != nil {
			return fmt.Errorf("invalid payload, decode failed: %w", err)
		}

		if err = s.BlockCommitAndDeliver(ctx, isLeader, b); err != nil {
			return fmt.Errorf("deliver and commit the tx block proposal failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type = %s", proposal.ProposalType)
	}

	if !isLeader {
		return nil
	}

	if err = s.RecordBlock(b); err != nil {
		return fmt.Errorf("record block failed: %w", err)
	}

	return nil
}

func (s *StaticRelayInsideOp) Close() {
	_ = s.RelayTxBlockOp.Close()
	_ = s.chain.Close()
}
