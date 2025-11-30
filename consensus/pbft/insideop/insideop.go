package insideop

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"

	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type ShardInsideOp interface {
	// BuildProposal build a proposal for a round of the PBFT consensus.
	// Note that, this function is normally called by the leader.
	// If both the returned proposal and the returned error are nil, the leader of PBFT should not propose now.
	BuildProposal(ctx context.Context) (*message.Proposal, error)
	ValidateProposal(ctx context.Context, proposal *message.Proposal) error
	ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error
	Close()
}

func WrapProposal(b *block.Block, proposalType string) (*message.Proposal, error) {
	var p message.Proposal

	payload, err := b.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode block payload: %w", err)
	}

	p.Payload = payload
	p.ProposalType = proposalType

	return &p, nil
}

// modifyTxRelayOpt sets the RelayOpt of txs
func modifyTxRelayOpt(ctx context.Context, txs []transaction.Transaction, ch *chain.Chain) ([]transaction.Transaction, error) {
	accountLocations, err := getAccountLocationsInTxs(ctx, ch, txs)
	if err != nil {
		return nil, fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	modifiedTxs := make([]transaction.Transaction, 0, len(txs))
	shardID := ch.GetShardID()

	for _, tx := range txs {
		// if this tx's relay stage is determined, not modify it
		if tx.RelayStage != transaction.UndeterminedRelayTx {
			modifiedTxs = append(modifiedTxs, tx)
			continue
		}

		senderID := accountLocations[tx.Sender]
		recipientID := accountLocations[tx.Recipient]

		if senderID < 0 || recipientID < 0 {
			return nil, fmt.Errorf("tx sender or recipient does not exist in the accountLocation map")
		}

		if senderID != shardID {
			slog.ErrorContext(ctx, "modify tx relay opt failed, the sender of this tx is not in this shard, and this transaction is not a relay-2 tx", "cur shardID", shardID, "expect shardID", senderID)
			continue
		}

		// this is an inner-shard tx, append it
		if senderID == recipientID {
			modifiedTxs = append(modifiedTxs, tx)
			continue
		}

		// this is a cross-shard tx, modify its RelayOpt
		var thash []byte

		if thash, err = utils.CalcHash(&tx); err != nil {
			return nil, fmt.Errorf("CalcHash failed: %w", err)
		}

		r1tx := tx
		r1tx.RelayStage = transaction.Relay1Tx
		r1tx.ROriginalHash = thash
		modifiedTxs = append(modifiedTxs, r1tx)
	}

	return modifiedTxs, nil
}

func getAccountLocationsInTxs(ctx context.Context, c *chain.Chain, txs []transaction.Transaction) (map[account.Account]int64, error) {
	// get all locations of accounts.
	accountLocations := make(map[account.Account]int64)
	for _, tx := range txs {
		accountLocations[tx.Sender] = -1
		accountLocations[tx.Recipient] = -1
	}

	requestAccounts := maps.Keys(accountLocations)

	states, err := c.GetAccountStates(ctx, requestAccounts)
	if err != nil {
		return nil, fmt.Errorf("GetAccountStates failed: %w", err)
	}

	for i, requestAccount := range requestAccounts {
		accountLocations[requestAccount] = states[i].ShardLocation
	}

	return accountLocations, nil
}
