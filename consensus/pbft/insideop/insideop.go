package insideop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/csvwrite"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
)

type ShardInsideOp interface {
	// BuildProposal build a proposal for a round of the PBFT consensus.
	// Note that, this function is normally called by the leader.
	// If both the returned proposal and the returned error are nil, the leader of PBFT should not propose now.
	BuildProposal(ctx context.Context) (*message.Proposal, error)
	// ValidateProposal validates the proposal.
	ValidateProposal(ctx context.Context, proposal *message.Proposal) error
	// ProposalCommitAndDeliver commits the given proposal and deliver the related messages to the supervisor or other shards.
	ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error
}

func recordBlock(caller *csvwrite.CSVSeqWriter, b *block.Block) error {
	line, err := block.ConvertBlock2Line(b)
	if err != nil {
		return fmt.Errorf("ConvertBlock2Line failed: %w", err)
	}

	if err = caller.WriteLine2CSV(line); err != nil {
		return fmt.Errorf("WriteLine2CSV failed: %w", err)
	}

	return nil
}
