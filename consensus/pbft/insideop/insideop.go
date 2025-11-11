package insideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type ShardInsideExtraOp interface {
	BuildProposal(ctx context.Context) (*message.Proposal, error)
	ValidateProposal(ctx context.Context, proposal *message.Proposal) (bool, error)
	ConfirmBlock(ctx context.Context) error
}
