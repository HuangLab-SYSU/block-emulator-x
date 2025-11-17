package insideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type CLPARelayInsideOp struct{}

func (C *CLPARelayInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// TODO implement me
	panic("implement me")
}

func (C *CLPARelayInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (C *CLPARelayInsideOp) ProposalCommitAndDeliver(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (C *CLPARelayInsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
