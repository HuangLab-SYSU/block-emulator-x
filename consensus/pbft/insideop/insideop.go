package insideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type ShardInsideOp interface {
	BuildProposal(ctx context.Context) (*message.Proposal, error)
	ValidateProposal(ctx context.Context, proposal *message.Proposal) error
	DeliverConfirmedProposal(ctx context.Context, proposal *message.Proposal) error
	Close()
}
