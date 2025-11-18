package insideop

import (
	"context"
	"fmt"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
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

	switch typedProposal := rawProposal.(type) {
	case *block.Block:
		payload, err := typedProposal.Encode()
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

func getAccountLocationsInTxs(ctx context.Context, c *chain.Chain, txs []transaction.Transaction) (map[account.Account]int64, error) {
	// get all locations of accounts.
	accountLocations := make(map[account.Account]int64)
	for _, tx := range txs {
		accountLocations[tx.Sender] = -1
		accountLocations[tx.Recipient] = -1
	}

	requestAccounts := maps.Keys(accountLocations)

	states, err := c.GetAccountLocations(ctx, requestAccounts)
	if err != nil {
		return nil, fmt.Errorf("GetAccountLocations failed: %w", err)
	}

	for i, requestAccount := range requestAccounts {
		if states[i] == nil {
			return nil, fmt.Errorf("unexpected error: state is nil for account: %s", requestAccounts[i])
		}

		accountLocations[requestAccount] = states[i].ShardLocation
	}

	return accountLocations, nil
}
