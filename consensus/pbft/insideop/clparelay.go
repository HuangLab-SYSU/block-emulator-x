package insideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
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

	blockCSVWriter

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewCLPARelayInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper,
	chain *chain.Chain, txPool txpool.TxPool, amm *migration.AccMigrateMetadata,
	cfg config.ConsensusNodeCfg, lp config.LocalParams,
) (*CLPARelayInsideOp, error) {
	bcw, err := newBlockCSVWriter(cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("newBlockCSVWriter failed: %w", err)
	}

	return &CLPARelayInsideOp{
		amm:            amm,
		conn:           conn,
		resolver:       resolver,
		chain:          chain,
		txPool:         txPool,
		blockCSVWriter: *bcw,
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

	var b *block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	if err := c.chain.ValidateBlock(ctx, b); err != nil {
		return fmt.Errorf("validate block failed: %w", err)
	}

	return nil
}

func (c *CLPARelayInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	switch proposal.ProposalType {
	case message.BlockProposalType:
		if err := c.txBlockCommitAndDeliver(ctx, isLeader, proposal); err != nil {
			return fmt.Errorf("deliver and commit the tx block proposal failed: %w", err)
		}
	case message.PartitionProposalType:
		if err := c.partitionBlockCommit(ctx, isLeader, proposal); err != nil {
			return fmt.Errorf("commit partition block failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type = %s", proposal.ProposalType)
	}

	return nil
}

func (c *CLPARelayInsideOp) Close() {
	_ = c.file.Close()
	_ = c.chain.Close()
}

func (c *CLPARelayInsideOp) buildBlockProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := c.packValidTxs(ctx, int(c.cfg.BlockSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	// if a transaction is a cross-shard tx, modify its RelayOpt
	mTxs, err := modifyTxRelayOpt(ctx, txs, c.chain)
	if err != nil {
		return nil, fmt.Errorf("modifyTxRelayOpt failed: %w", err)
	}

	b, err := c.chain.GenerateBlock(ctx, c.lp.WalletAddr, mTxs)
	if err != nil {
		return nil, fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	p, err := WrapProposal(b, message.BlockProposalType)
	if err != nil {
		return nil, fmt.Errorf("WrapProposal failed: %w", err)
	}

	slog.InfoContext(ctx, "block is generated in clpa relay module", "shard ID", c.chain.GetShardID(), "block height", b.Header.Number, "block create time", b.Header.CreateTime)

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

	p, err := WrapProposal(b, message.PartitionProposalType)
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
		accountLoc, err := getAccountLocationsInTxs(ctx, c.chain, txs)
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
	accountsMigratedOut := maps.Keys(c.amm.CurModifiedMap)

	states, err := c.chain.GetAccountStates(ctx, accountsMigratedOut)
	if err != nil {
		return fmt.Errorf("GetAccountStates failed: %w", err)
	}

	allTxs, err := c.txPool.PackTxs(migratedTxNum)
	if err != nil {
		return fmt.Errorf("PackTxs failed: %w", err)
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
		// If this account is not in this shard, skip it
		if states[i].ShardLocation != c.lp.ShardID {
			continue
		}

		destShardID := c.amm.CurModifiedMap[acc]
		if int64(destShardID) == c.lp.ShardID { // If this dest shard is it, skip it
			continue
		}

		atMsgList[destShardID].AccountStates[acc] = states[i]
	}

	addBackTxs := make([]transaction.Transaction, 0)

	for _, tx := range allTxs {
		senderDestShard, senderModified := c.amm.CurModifiedMap[tx.Sender]
		recipientDestShard, recipientModified := c.amm.CurModifiedMap[tx.Recipient]
		// If this transaction is a relay-2 transaction, it should be sent to the recipient's shard
		if tx.RelayStage == transaction.Relay2Tx {
			if recipientModified {
				atMsgList[recipientDestShard].MigratedTxs = append(atMsgList[recipientDestShard].MigratedTxs, tx)
			} else {
				addBackTxs = append(addBackTxs, tx)
			}
		} else { // Otherwise, it should be sent to the sender's shard.
			if senderModified {
				atMsgList[senderDestShard].MigratedTxs = append(atMsgList[senderDestShard].MigratedTxs, tx)
			} else {
				addBackTxs = append(addBackTxs, tx)
			}
		}
	}

	// Add the unmigrated txs back to the tx pool.
	if err = c.txPool.AddTxs(addBackTxs); err != nil {
		return fmt.Errorf("AddTxs failed: %w", err)
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

func (c *CLPARelayInsideOp) txBlockCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	var b block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode as block failed: %w", err)
	}
	// commit block - add block to the blockchain
	if err := c.chain.AddBlock(ctx, &b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	slog.Info("block is added in clpa relay module", "block height", b.Header.Number, "epoch", c.amm.Epoch)

	// if this node is not a leader, skip
	if !isLeader {
		return nil
	}

	// record this block
	line, err := convertBlock2Line(&b)
	if err != nil {
		return fmt.Errorf("convertBlock2Line failed: %w", err)
	}

	if err = utils.WriteLine2CSV(c.csvW, line); err != nil {
		return fmt.Errorf("WriteLine2CSV failed: %w", err)
	}

	// deliver this block info to the supervisor
	innerTxs, r1Txs, r2Txs := c.splitTxs(ctx, b.TxList)

	if err = c.deliverBlockInfo2Supervisor(ctx, innerTxs, r1Txs, r2Txs, b); err != nil {
		return fmt.Errorf("deliverBlockInfo2Supervisor failed: %w", err)
	}

	accountLocations, err := getAccountLocationsInTxs(ctx, c.chain, b.TxList)
	if err != nil {
		return fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	if err = c.sendRelayedTxs(ctx, r1Txs, accountLocations); err != nil {
		return fmt.Errorf("sendRelayedTxs failed: %w", err)
	}

	return nil
}

func (c *CLPARelayInsideOp) partitionBlockCommit(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	var b block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode as block failed: %w", err)
	}
	// commit block - add block to the blockchain
	if err := c.chain.AddBlock(ctx, &b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	c.chain.UpdateEpoch(c.amm.Epoch)
	c.amm.MigrationStatusReset()
	slog.Info("block is added in clpa relay module", "block height", b.Header.Number)

	if isLeader {
		return nil
	}
	// record this block
	line, err := convertBlock2Line(&b)
	if err != nil {
		return fmt.Errorf("convertBlock2Line failed: %w", err)
	}

	if err = utils.WriteLine2CSV(c.csvW, line); err != nil {
		return fmt.Errorf("WriteLine2CSV failed: %w", err)
	}

	return nil
}

// splitTxs split transactions to inner-shard txs, relay1 txs and relay2 txs.
func (c *CLPARelayInsideOp) splitTxs(ctx context.Context, txs []transaction.Transaction) ([]transaction.Transaction, []transaction.Transaction, []transaction.Transaction) {
	innerTxs, r1txs, r2txs := make([]transaction.Transaction, 0), make([]transaction.Transaction, 0), make([]transaction.Transaction, 0)

	for _, tx := range txs {
		switch tx.RelayStage {
		case transaction.UndeterminedRelayTx:
			innerTxs = append(innerTxs, tx)
		case transaction.Relay1Tx:
			r1txs = append(r1txs, tx)
		case transaction.Relay2Tx:
			r2txs = append(r2txs, tx)
		default:
			slog.ErrorContext(ctx, "invalid relay tx stage", "relay stage", tx.RelayStage)
		}
	}

	return innerTxs, r1txs, r2txs
}

func (c *CLPARelayInsideOp) deliverBlockInfo2Supervisor(ctx context.Context, innerTxs, r1Txs, r2Txs []transaction.Transaction, b block.Block) error {
	rbm := &message.RelayBlockInfoMsg{
		InnerShardTxs:    innerTxs,
		Relay1Txs:        r1Txs,
		Relay2Txs:        r2Txs,
		ShardID:          c.chain.GetShardID(),
		Epoch:            c.chain.GetEpochID(),
		BlockProposeTime: b.Header.CreateTime,
		BlockCommitTime:  time.Now(),
	}

	w, err := message.WrapMsg(rbm)
	if err != nil {
		return fmt.Errorf("WrapMsg failed: %w", err)
	}

	spv, err := c.resolver.GetSupervisor()
	if err != nil {
		return fmt.Errorf("GetSupervisor failed: %w", err)
	}

	go c.conn.SendMessage(ctx, spv, w)

	return nil
}

func (c *CLPARelayInsideOp) sendRelayedTxs(ctx context.Context, r1Txs []transaction.Transaction, accountLocations map[account.Account]int64) error {
	// for relay1 txs, send relay messages to other shards.
	relayedTxs := make([][]transaction.Transaction, c.cfg.ShardNum)

	// split r1Txs into all shards
	for _, tx := range r1Txs {
		// the next destination of relay1 tx should be calculated according to the recipient addr.
		shardID, ok := accountLocations[tx.Recipient]
		if !ok {
			slog.ErrorContext(ctx, "tx.Recipient is not found in accountLocations", "recipient", tx.Recipient)
			continue
		}

		// modify relay transaction's RelayOpt
		updatedRelayedTx := tx
		updatedRelayedTx.RelayStage = transaction.Relay2Tx
		relayedTxs[shardID] = append(relayedTxs[shardID], updatedRelayedTx)
	}

	node2Msg := make(map[nodetopo.NodeInfo]*rpcserver.WrappedMsg, c.cfg.ShardNum)

	// pack messages and send them
	for i, txs := range relayedTxs {
		if len(txs) == 0 {
			continue
		}

		l, err := c.resolver.GetLeader(int64(i))
		if err != nil {
			return fmt.Errorf("GetLeader failed: %w", err)
		}

		w, err := message.WrapMsg(&message.ReceiveTxsMsg{Txs: txs})
		if err != nil {
			return fmt.Errorf("WrapMsg failed: %w", err)
		}

		node2Msg[l] = w
	}

	c.conn.MSendDifferentMessages(ctx, node2Msg)

	return nil
}
