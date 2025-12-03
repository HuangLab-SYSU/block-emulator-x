package insideop

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

const migratedTxNum = 1 << 28

type CLPARelayInsideOp struct {
	amm *migration.AccMigrateMetadata

	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.

	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	*txblockop.RelayTxBlockOp

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewCLPARelayInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper,
	chain *chain.Chain, txPool txpool.TxPool, amm *migration.AccMigrateMetadata,
	cfg config.ConsensusNodeCfg, lp config.LocalParams,
) (*CLPARelayInsideOp, error) {
	rh, err := txblockop.NewRelayTxBlockOp(chain, conn, resolver, cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewRelayTxBlockOp failed: %w", err)
	}

	return &CLPARelayInsideOp{
		amm:            amm,
		conn:           conn,
		resolver:       resolver,
		chain:          chain,
		txPool:         txPool,
		RelayTxBlockOp: rh,
		cfg:            cfg,
		lp:             lp,
	}, nil
}

func (c *CLPARelayInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// if MigrationReady is not ready, propose a transaction block.
	if !c.amm.MigrationReady {
		return c.buildBlockProposal(ctx)
	}

	// MigrationReady is true, thus this node is ready to propose a partition block.
	return c.buildPartitionProposal(ctx)
}

func (c *CLPARelayInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	if proposal.ProposalType != message.BlockProposalType && proposal.ProposalType != message.PartitionProposalType {
		return fmt.Errorf("invalid proposal type")
	}

	b, err := block.DecodeBlock(proposal.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	if err = c.chain.ValidateBlock(ctx, b); err != nil {
		return fmt.Errorf("validate block failed: %w", err)
	}

	return nil
}

func (c *CLPARelayInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	var (
		b   *block.Block
		err error
	)

	switch proposal.ProposalType {
	case message.BlockProposalType:
		b, err = block.DecodeBlock(proposal.Payload)
		if err != nil {
			return fmt.Errorf("invalid payload, decode failed: %w", err)
		}

		if err = c.BlockCommitAndDeliver(ctx, isLeader, b); err != nil {
			return fmt.Errorf("deliver and commit the tx block proposal failed: %w", err)
		}
	case message.PartitionProposalType:
		b, err = block.DecodeBlock(proposal.Payload)
		if err != nil {
			return fmt.Errorf("invalid payload, decode failed: %w", err)
		}

		if err = c.partitionBlockCommit(ctx, proposal); err != nil {
			return fmt.Errorf("commit partition block failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type = %s", proposal.ProposalType)
	}

	if !isLeader {
		return nil
	}

	if err = c.RecordBlock(b); err != nil {
		return fmt.Errorf("record block failed: %w", err)
	}

	return nil
}

func (c *CLPARelayInsideOp) Close() {
	_ = c.RelayTxBlockOp.Close()
	_ = c.chain.Close()
}

func (c *CLPARelayInsideOp) buildBlockProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := c.packValidTxs(ctx, int(c.cfg.BlockSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	p, err := c.BuildTxBlockProposal(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("build block proposal in relay mode failed: %w", err)
	}

	return p, nil
}

func (c *CLPARelayInsideOp) buildPartitionProposal(ctx context.Context) (*message.Proposal, error) {
	// If the account-migration message is not sent, send it to all shards.
	if _, msgSent := c.amm.MigratedAccountStates[c.lp.ShardID]; !msgSent {
		if err := c.migrateAccountsAndTxs(ctx); err != nil {
			return nil, fmt.Errorf("migrateAccountsAndTxs failed: %w", err)
		}

		slog.InfoContext(ctx, "accounts and txs has been migrated in this epoch", "shard", c.lp.ShardID, "epoch", c.amm.Epoch)
	}

	// If the number of received account-migration messages is enough (equal to ShardNum),
	// the leader will pack the partition proposal in a block.
	if len(c.amm.MigratedAccountStates) != int(c.cfg.ShardNum) {
		slog.Info("not all MigratedAccountStates is collected, do not propose", "expect", int(c.cfg.ShardNum), "actual", len(c.amm.MigratedAccountStates))
		return nil, nil
	}

	accounts, states, err := c.amm.GetMigratedAccountsAndStates()
	if err != nil {
		return nil, fmt.Errorf("GetMigratedAccountsAndStates failed: %w", err)
	}

	b, err := c.chain.GenerateMigrationBlock(ctx, c.lp.WalletAddr, accounts, states)
	if err != nil {
		return nil, fmt.Errorf("GenerateMigrationBlock failed: %w", err)
	}

	p, err := message.WrapProposal(b, message.PartitionProposalType)
	if err != nil {
		return nil, fmt.Errorf("WrapProposal failed: %w", err)
	}

	return p, nil
}

// packValidTxs packs transactions from the tx pool.
// Because of the account migration, if a transaction should not be processed in this shard,
// it wil also be migrated to the correct shard.
func (c *CLPARelayInsideOp) packValidTxs(ctx context.Context, size int) ([]transaction.Transaction, error) {
	txsPacked := make([]transaction.Transaction, 0)
	txs2Shard := make([][]transaction.Transaction, c.cfg.ShardNum)

	for curCnt := 0; curCnt < size; {
		iterPackedTxs := make([]transaction.Transaction, 0)

		txs, err := c.txPool.PackTxs(size - curCnt)
		if err != nil {
			return nil, fmt.Errorf("PackTxs failed: %w", err)
		}

		if len(txs) == 0 { // no transactions to pack, break
			break
		}
		// Get all account states of this txs.
		accountLoc, err := c.chain.GetAccountLocationsInTxs(ctx, txs)
		if err != nil {
			return nil, fmt.Errorf("GetAccountLocationsInTxs failed: %w", err)
		}

		for _, tx := range txs {
			keyAcc := tx.Sender
			if tx.RelayStage == transaction.Relay2Tx {
				keyAcc = tx.Recipient
			}

			destShard, ok := accountLoc[keyAcc]
			if !ok {
				slog.Error("account not found in accountLoc", "account", keyAcc)
				continue
			}

			if destShard != c.lp.ShardID {
				txs2Shard[destShard] = append(txs2Shard[destShard], tx)
			} else {
				iterPackedTxs = append(iterPackedTxs, tx)
			}
		}

		incSize, err := c.txPool.GetTxListSize(iterPackedTxs)
		if err != nil {
			return nil, fmt.Errorf("GetTxListSize failed: %w", err)
		}

		curCnt += incSize

		txsPacked = append(txsPacked, iterPackedTxs...)
	}

	sendMsgMap := make(map[nodetopo.NodeInfo]*rpcserver.WrappedMsg, len(txs2Shard))
	for i := range txs2Shard {
		if int64(i) == c.lp.ShardID || len(txs2Shard[i]) == 0 {
			continue
		}

		l, err := c.resolver.GetLeader(int64(i))
		if err != nil {
			return nil, fmt.Errorf("GetLeader failed: %w", err)
		}

		w, err := message.WrapMsg(&message.ReceiveTxsMsg{Txs: txs2Shard[i]})
		if err != nil {
			return nil, fmt.Errorf("WrapMsg failed: %w", err)
		}

		slog.Info("migrate transactions to the other shard", "destShard", i, "size", len(txs2Shard[i]))

		sendMsgMap[l] = w
	}

	c.conn.MSendDifferentMessages(ctx, sendMsgMap)

	return txsPacked, nil
}

// migrateAccountsAndTxs collects accounts and txs and sends them to other shards.
// The account states are fetched from the chain and the txs are fetched from the txPool.
func (c *CLPARelayInsideOp) migrateAccountsAndTxs(ctx context.Context) error {
	if err := c.migrateAccounts(ctx); err != nil {
		return fmt.Errorf("migrateAccounts failed: %w", err)
	}

	if err := c.migrateTxs(ctx); err != nil {
		return fmt.Errorf("migrateTxs failed: %w", err)
	}

	return nil
}

// migrateAccounts migrates accounts.
func (c *CLPARelayInsideOp) migrateAccounts(ctx context.Context) error {
	accountsMigratedOut := maps.Keys(c.amm.CurModifiedMap)

	states, err := c.chain.GetAccountStates(ctx, accountsMigratedOut)
	if err != nil {
		return fmt.Errorf("GetAccountStates failed: %w", err)
	}

	atMsgList := make([]message.AccountAndTxMigrationMsg, c.cfg.ShardNum)

	// Init the AccountAndTxMigrationMsg list
	for i := range atMsgList {
		atMsgList[i] = message.AccountAndTxMigrationMsg{
			SrcShard:      c.lp.ShardID,
			DestShard:     int64(i),
			Epoch:         c.amm.Epoch,
			AccountStates: make(map[account.Account]*account.State),
		}
	}

	for i, acc := range accountsMigratedOut {
		// If this shard is in this shard now, add this account to the dest shard.
		srcShardID := states[i].ShardLocation

		destShardID := int64(c.amm.CurModifiedMap[acc])
		if srcShardID == c.lp.ShardID && destShardID != c.lp.ShardID {
			atMsgList[destShardID].AccountStates[acc] = states[i]
		}
	}

	sendMsgMap := make(map[nodetopo.NodeInfo]*rpcserver.WrappedMsg, len(atMsgList))
	for i := range atMsgList {
		l, err := c.resolver.GetLeader(int64(i))
		if err != nil {
			return fmt.Errorf("GetLeader failed: %w", err)
		}

		w, err := message.WrapMsg(&atMsgList[i])
		if err != nil {
			return fmt.Errorf("WrapMsg failed: %w", err)
		}

		sendMsgMap[l] = w
	}

	c.conn.MSendDifferentMessages(ctx, sendMsgMap)

	return nil
}

// migrateTxs migrates transactions.
func (c *CLPARelayInsideOp) migrateTxs(ctx context.Context) error {
	allTxs, err := c.txPool.PackTxs(migratedTxNum)
	if err != nil {
		return fmt.Errorf("PackTxs failed: %w", err)
	}

	tx2Shards := make([][]transaction.Transaction, c.cfg.ShardNum)
	addBackTxs := make([]transaction.Transaction, 0)

	for _, tx := range allTxs {
		destShard, modified := c.amm.CurModifiedMap[tx.Sender]
		if tx.RelayStage == transaction.Relay2Tx {
			// If this transaction is a relay-2 transaction, it should be sent to the recipient's shard
			destShard, modified = c.amm.CurModifiedMap[tx.Recipient]
		}

		if modified {
			tx2Shards[destShard] = append(tx2Shards[destShard], tx)
		} else {
			addBackTxs = append(addBackTxs, tx)
		}
	}

	// Add the unmigrated txs back to the tx pool.
	if err = c.txPool.AddTxs(addBackTxs); err != nil {
		return fmt.Errorf("AddTxs failed: %w", err)
	}

	if err = message.SendWrappedTxs2Shards(ctx, tx2Shards, c.conn, c.resolver); err != nil {
		return fmt.Errorf("SendWrappedTxs2Shards failed: %w", err)
	}

	return nil
}

func (c *CLPARelayInsideOp) partitionBlockCommit(ctx context.Context, proposal *message.Proposal) error {
	b, err := block.DecodeBlock(proposal.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}
	// commit block - add block to the blockchain
	if err = c.chain.AddBlock(ctx, b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	c.chain.UpdateEpoch(c.amm.Epoch)
	c.amm.MigrationStatusReset()
	slog.Info("block is added in clpa relay module", "block height", b.Number)

	return nil
}
