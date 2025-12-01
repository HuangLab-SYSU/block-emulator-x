package insideop

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator/config"

	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"

	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

const blockRecordPathFmt = "shard=%d_node=%d/block_record.csv"

var blockRecordHeader = []string{
	"ParentHash",
	"BlockHash",
	"StateRoot",
	"Number",
	"CreateTime",
	"TxRoot",
	"TxBodyLen",
	"MigratedAccountsRoot",
	"MigrationAccountLen",
}

type ShardInsideOp interface {
	// BuildProposal build a proposal for a round of the PBFT consensus.
	// Note that, this function is normally called by the leader.
	// If both the returned proposal and the returned error are nil, the leader of PBFT should not propose now.
	BuildProposal(ctx context.Context) (*message.Proposal, error)
	// ValidateProposal validates the proposal.
	ValidateProposal(ctx context.Context, proposal *message.Proposal) error
	// ProposalCommitAndDeliver commits the given proposal and deliver the related messages to the supervisor or other shards.
	ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error
	Close()
}

type blockCSVWriter struct {
	file *os.File
	csvW *csv.Writer
}

func newBlockCSVWriter(cfg config.ConsensusNodeCfg, lp config.LocalParams) (*blockCSVWriter, error) {
	fp := filepath.Join(cfg.BlockRecordDir, fmt.Sprintf(blockRecordPathFmt, lp.ShardID, lp.NodeID))

	file, err := utils.CreateFileWithDirs(fp)
	if err != nil {
		return nil, fmt.Errorf("CreateFileWithDirs failed: %w", err)
	}

	csvW := csv.NewWriter(file)
	if err = utils.WriteLine2CSV(csvW, blockRecordHeader); err != nil {
		return nil, fmt.Errorf("WriteLine2CSV failed: %w", err)
	}

	return &blockCSVWriter{file: file, csvW: csvW}, nil
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

// modifyTxRelayOpt sets the RelayOpt of txs.
func modifyTxRelayOpt(ctx context.Context, txs []transaction.Transaction, ch *chain.Chain) ([]transaction.Transaction, error) {
	accountLocations, err := getAccountLocationsInTxs(ctx, ch, txs)
	if err != nil {
		return nil, fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	modifiedTxs := make([]transaction.Transaction, 0, len(txs))
	shardID := ch.GetShardID()

	for _, tx := range txs {
		// if this transaction's relay stage is determined, not modify it
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

		if thash, err = tx.Hash(); err != nil {
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

func convertBlock2Line(b *block.Block) ([]string, error) {
	blockHash, err := b.Hash()
	if err != nil {
		return nil, fmt.Errorf("CalcHash failed: %w", err)
	}

	return []string{
		hex.EncodeToString(b.ParentBlockHash),      // "ParentHash"
		hex.EncodeToString(blockHash),              // "BlockHash"
		hex.EncodeToString(b.StateRoot),            // "StateRoot"
		fmt.Sprintf("%d", b.Number),                // "Number"
		utils.ConvertTime2Str(b.CreateTime),        // "CreateTime"
		hex.EncodeToString(b.TxRoot),               // "TxRoot"
		fmt.Sprintf("%d", len(b.TxList)),           // "TxBodyLen"
		hex.EncodeToString(b.MigratedAccountsRoot), // "MigratedAccountsRoot"
		fmt.Sprintf("%d", len(b.MigratedAccounts)), // "MigrationAccountLen"
	}, nil
}
