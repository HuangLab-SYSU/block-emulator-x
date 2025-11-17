package insideop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type ShardInsideOp interface {
	BuildProposal(ctx context.Context) (*message.Proposal, error)
	ValidateProposal(ctx context.Context, proposal *message.Proposal) error
	ProposalCommitAndDeliver(ctx context.Context, proposal *message.Proposal) error
	Close()
}

func WrapProposal(rawProposal any) (*message.Proposal, error) {
	var p message.Proposal

	switch rawProposal := rawProposal.(type) {
	case *block.Block:
		payload, err := rawProposal.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode block payload: %w", err)
		}

		p.Payload = payload
		p.ProposalType = message.BlockProposalType
	default:
		return nil, fmt.Errorf("unknown type: %T", rawProposal)
	}

	return &p, nil
}
