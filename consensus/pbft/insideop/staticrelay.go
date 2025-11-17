package insideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type StaticRelayInsideOp struct {
	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.
}

func (s *StaticRelayInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// TODO implement me
	panic("implement me")
}

func (s *StaticRelayInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	if proposal.ProposalType != message.BlockProposalType {
		return fmt.Errorf("invalid proposal type")
	}

	var b *block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	return nil
}

func (s *StaticRelayInsideOp) DeliverConfirmedProposal(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (s *StaticRelayInsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
