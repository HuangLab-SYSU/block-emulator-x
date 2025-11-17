package insideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type StaticBrokerInsideOp struct{}

func (s *StaticBrokerInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// TODO implement me
	panic("implement me")
}

func (s *StaticBrokerInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (s *StaticBrokerInsideOp) DeliverConfirmedProposal(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (s *StaticBrokerInsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
