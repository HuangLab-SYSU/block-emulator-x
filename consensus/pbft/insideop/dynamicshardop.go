package insideop

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/insideop/migrationblockop"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/csvwrite"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

const (
	migratedTxNum     = 1 << 28
	supervisorShardID = 0x7fffffff
)

type DynamicShardOp struct {
	amm *migration.AccMigrateMetadata

	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.

	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	tbo txblockop.TxBlockOp
	mbo *migrationblockop.MigrationBlockOp
	csw *csvwrite.CSVSeqWriter

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewDynamicShardOp(conn *network.P2PConn, resolver nodetopo.NodeMapper,
	chain *chain.Chain, txPool txpool.TxPool, amm *migration.AccMigrateMetadata,
	cfg config.ConsensusNodeCfg, lp config.LocalParams,
) (*DynamicShardOp, error) {
	tbo, err := txblockop.NewTxBlockOp(conn, resolver, chain, cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewTxBlockOp: %w", err)
	}

	fp := filepath.Join(cfg.BlockRecordDir, fmt.Sprintf(blockRecordPathFmt, lp.ShardID, lp.NodeID))

	csw, err := csvwrite.NewCSVSeqWriter(fp, block.RecordTitle)
	if err != nil {
		return nil, fmt.Errorf("NewCSVSeqWriter: %w", err)
	}

	return &DynamicShardOp{
		amm:      amm,
		conn:     conn,
		resolver: resolver,
		chain:    chain,
		txPool:   txPool,
		tbo:      tbo,
		mbo:      migrationblockop.NewMigrationBlockOp(conn, resolver, chain, amm, cfg, lp),
		csw:      csw,
		cfg:      cfg,
		lp:       lp,
	}, nil
}

func (c *DynamicShardOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// if MigrationReady is not ready, propose a transaction block.
	if !c.amm.MigrationReady {
		return c.buildBlockProposal(ctx)
	}

	// MigrationReady is true, thus this node is ready to propose a partition block.
	return c.buildPartitionProposal(ctx)
}

func (c *DynamicShardOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
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

func (c *DynamicShardOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	b, err := block.DecodeBlock(proposal.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	switch proposal.ProposalType {
	case message.BlockProposalType:
		if err = c.tbo.BlockCommitAndDeliver(ctx, isLeader, b); err != nil {
			return fmt.Errorf("block commit failed: %w", err)
		}
	case message.PartitionProposalType:
		if err = c.mbo.MigrationBlockCommit(ctx, isLeader, b); err != nil {
			return fmt.Errorf("migration block commit failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type=%s", proposal.ProposalType)
	}

	if isLeader {
		if err = recordBlock(c.csw, b); err != nil {
			return fmt.Errorf("record block failed: %w", err)
		}
	}

	return nil
}

func (c *DynamicShardOp) Close() {
	_ = c.csw.Close()
	_ = c.chain.Close()
}

func (c *DynamicShardOp) buildBlockProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := c.packValidTxs(ctx, int(c.cfg.BlockSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	p, err := c.tbo.BuildTxBlockProposal(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("build block proposal in relay mode failed: %w", err)
	}

	return p, nil
}

func (c *DynamicShardOp) buildPartitionProposal(ctx context.Context) (*message.Proposal, error) {
	// If the account-migration message is not sent, send it to all shards.
	if _, msgSent := c.amm.MigratedAccountStates[c.lp.ShardID]; !msgSent {
		if err := c.mbo.MigrateAccounts(ctx); err != nil {
			return nil, fmt.Errorf("migrateAccounts failed: %w", err)
		}

		if err := c.migrateTxs(ctx); err != nil {
			return nil, fmt.Errorf("migrateTxs failed: %w", err)
		}

		slog.InfoContext(ctx, "accounts and txs has been migrated in this epoch", "shard", c.lp.ShardID, "epoch", c.amm.Epoch)
	}

	p, err := c.mbo.BuildMigrationProposal(ctx)
	if err != nil {
		return nil, fmt.Errorf("build migration proposal failed: %w", err)
	}

	return p, nil
}

// packValidTxs packs transactions from the tx pool.
// Because of the account migration, if a transaction should not be processed in this shard,
// it wil also be migrated to the correct shard.
func (c *DynamicShardOp) packValidTxs(ctx context.Context, size int) ([]transaction.Transaction, error) {
	if c.cfg.ConsensusType == config.CLPARelayConsensus {
		return c.packValidTxsInRelay(ctx, size)
	}

	return c.packValidTxsInBroker(ctx, size)
}

func (c *DynamicShardOp) packValidTxsInRelay(ctx context.Context, size int) ([]transaction.Transaction, error) {
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

	if err := message.SendWrappedTxs2Shards(ctx, txs2Shard, c.conn, c.resolver); err != nil {
		return nil, fmt.Errorf("SendWrappedTxs2Shard failed: %w", err)
	}

	return txsPacked, nil
}

func (c *DynamicShardOp) packValidTxsInBroker(ctx context.Context, size int) ([]transaction.Transaction, error) {
	txsPacked := make([]transaction.Transaction, 0)
	txs2Supervisor := make([]transaction.Transaction, 0)
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
			if destShard := c.getTxDestLocByAccountState(tx, accountLoc); destShard == supervisorShardID {
				txs2Supervisor = append(txs2Supervisor, tx)
			} else if destShard == c.lp.ShardID {
				iterPackedTxs = append(iterPackedTxs, tx)
			} else {
				txs2Shard[destShard] = append(txs2Shard[destShard], tx)
			}
		}

		incSize, err := c.txPool.GetTxListSize(iterPackedTxs)
		if err != nil {
			return nil, fmt.Errorf("GetTxListSize failed: %w", err)
		}

		curCnt += incSize

		txsPacked = append(txsPacked, iterPackedTxs...)
	}

	if err := message.SendWrappedTxs2Shards(ctx, txs2Shard, c.conn, c.resolver); err != nil {
		return nil, fmt.Errorf("SendWrappedTxs2Shard failed: %w", err)
	}

	if err := c.brokerCLPATxSendAgain(ctx, txs2Supervisor); err != nil {
		return nil, fmt.Errorf("brokerCLPATxSendAgain failed: %w", err)
	}

	return txsPacked, nil
}

// migrateTxs migrates transactions.
func (c *DynamicShardOp) migrateTxs(ctx context.Context) error {
	if c.cfg.ConsensusType == config.CLPARelayConsensus {
		return c.migrateTxsInRelay(ctx)
	}

	return c.migrateTxsInBroker(ctx)
}

func (c *DynamicShardOp) migrateTxsInRelay(ctx context.Context) error {
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

func (c *DynamicShardOp) migrateTxsInBroker(ctx context.Context) error {
	allTxs, err := c.txPool.PackTxs(migratedTxNum)
	if err != nil {
		return fmt.Errorf("PackTxs failed: %w", err)
	}

	tx2Shards := make([][]transaction.Transaction, c.cfg.ShardNum)
	addBackTxs := make([]transaction.Transaction, 0)
	tx2Supervisor := make([]transaction.Transaction, 0)

	for _, tx := range allTxs {
		if dest := c.getTxDestLocByModifiedMap(tx); dest == supervisorShardID {
			tx2Supervisor = append(tx2Supervisor, tx)
		} else if dest == c.lp.ShardID {
			addBackTxs = append(addBackTxs, tx)
		} else {
			tx2Shards[dest] = append(tx2Shards[dest], tx)
		}
	}

	// Add the unmigrated txs back to the tx pool.
	if err = c.txPool.AddTxs(addBackTxs); err != nil {
		return fmt.Errorf("AddTxs failed: %w", err)
	}

	if err = message.SendWrappedTxs2Shards(ctx, tx2Shards, c.conn, c.resolver); err != nil {
		return fmt.Errorf("SendWrappedTxs2Shards failed: %w", err)
	}

	if err = c.brokerCLPATxSendAgain(ctx, tx2Supervisor); err != nil {
		return fmt.Errorf("brokerCLPATxSendAgain failed: %w", err)
	}

	return nil
}

func (c *DynamicShardOp) getTxDestLocByModifiedMap(tx transaction.Transaction) int64 {
	sDestShard, sModified := c.amm.CurModifiedMap[tx.Sender]
	rDestShard, rModified := c.amm.CurModifiedMap[tx.Recipient]

	if len(tx.BOriginalHash) == 0 { // inner-shard tx
		if !sModified {
			sDestShard = int(c.lp.ShardID)
		}

		if !rModified {
			rDestShard = int(c.lp.ShardID)
		}
		// After the account-migration, this transaction is still an inner-shard tx. Thus, it should be migrated into sDestShard.
		if sDestShard == rDestShard {
			return int64(sDestShard)
		}
		// After the account-migration, this transaction is a cross-shard tx. Thus, it should be migrated into supervisorShard.
		return supervisorShardID
	}

	if tx.BrokerStage == transaction.Sigma1BrokerStage { // broker-1 tx
		// After the account-migration, this transaction should be set in sDestShard.
		if !sModified {
			sDestShard = int(c.lp.ShardID)
		}

		return int64(sDestShard)
	}

	if tx.BrokerStage == transaction.Sigma2BrokerStage { // broker-2 tx
		// After the account-migration, this transaction should be set in rDestShard.
		if !rModified {
			rDestShard = int(c.lp.ShardID)
		}

		return int64(rDestShard)
	}

	slog.Error("invalid broker stage", "stage", tx.BrokerStage)

	return supervisorShardID
}

func (c *DynamicShardOp) getTxDestLocByAccountState(tx transaction.Transaction, accountLoc map[account.Account]int64) int64 {
	sDestShard, sModified := accountLoc[tx.Sender]
	rDestShard, rModified := accountLoc[tx.Recipient]

	if !sModified || !rModified {
		slog.Error("sender or recipient is not found in accountLoc", "sender", tx.Sender, "recipient", tx.Recipient)
		return supervisorShardID
	}

	if len(tx.BOriginalHash) == 0 { // inner-shard tx
		// After the account-migration, this transaction is still an inner-shard tx. Thus, it should be migrated into sDestShard.
		if sDestShard == rDestShard {
			return sDestShard
		}
		// After the account-migration, this transaction is a cross-shard tx. Thus, it should be migrated into supervisorShard.
		return supervisorShardID
	}

	if tx.BrokerStage == transaction.Sigma1BrokerStage { // broker-1 tx
		// After the account-migration, this transaction should be set in sDestShard.
		return sDestShard
	}

	if tx.BrokerStage == transaction.Sigma2BrokerStage { // broker-2 tx
		// After the account-migration, this transaction should be set in rDestShard.
		return rDestShard
	}

	slog.Error("invalid broker stage", "stage", tx.BrokerStage)

	return supervisorShardID
}

func (c *DynamicShardOp) brokerCLPATxSendAgain(ctx context.Context, txSentAgain []transaction.Transaction) error {
	// Send to the supervisor
	w, err := message.WrapMsg(&message.BrokerCLPATxSendAgainMsg{Txs: txSentAgain})
	if err != nil {
		return fmt.Errorf("wrap BrokerCLPATxSendAgainMsg failed: %w", err)
	}

	s, err := c.resolver.GetSupervisor()
	if err != nil {
		return fmt.Errorf("GetSupervisor failed: %w", err)
	}

	go c.conn.SendMessage(ctx, s, w)

	return nil
}
