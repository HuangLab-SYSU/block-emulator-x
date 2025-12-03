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

type StaticBrokerInsideOp struct {
	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	*txblockop.BrokerTxBlockOp

	cfg config.ConsensusNodeCfg
}

func NewStaticBrokerInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper, chain *chain.Chain, txPool txpool.TxPool, cfg config.ConsensusNodeCfg, lp config.LocalParams) (*StaticBrokerInsideOp, error) {
	bh, err := txblockop.NewBrokerTxBlockOp(chain, conn, resolver, cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewBrokerTxBlockOp failed: %w", err)
	}

	return &StaticBrokerInsideOp{
		chain:           chain,
		txPool:          txPool,
		BrokerTxBlockOp: bh,
		cfg:             cfg,
	}, nil
}

func (s *StaticBrokerInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := s.txPool.PackTxs(int(s.cfg.BlockSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	p, err := s.BuildTxBlockProposal(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("BuildTxBlockProposal failed: %w", err)
	}

	return p, nil
}

func (s *StaticBrokerInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
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

// ProposalCommitAndDeliver of StaticBrokerInsideOp contains:
// 1. apply the proposal to the chain.
// 2. send blockInfoMsg to the supervisor.
func (s *StaticBrokerInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
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

	if err = s.RecordBlock(b); err != nil {
		return fmt.Errorf("recordBlock failed: %w", err)
	}

	return nil
}

func (s *StaticBrokerInsideOp) Close() {
	_ = s.BrokerTxBlockOp.Close()
	_ = s.chain.Close()
}
