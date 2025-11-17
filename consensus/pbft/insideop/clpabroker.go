package insideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type CLPABrokerInsideOp struct{}

func (C *CLPABrokerInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// TODO implement me
	panic("implement me")
}

func (C *CLPABrokerInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (C *CLPABrokerInsideOp) ProposalCommitAndDeliver(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (C *CLPABrokerInsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
