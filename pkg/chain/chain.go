package chain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/bloom"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator/pkg/storage"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

type Chain struct {
	s         *storage.Storage
	curHeader *block.Header
	shardID   int64
	epochID   int64

	cfg config.BlockchainCfg

	mux sync.Mutex
}

// NewChain creates a new blockchain data structure with given components.
func NewChain(cfg config.BlockchainCfg, lp config.LocalParams) (*Chain, error) {
	s, err := storage.NewStorage(cfg.StorageCfg, lp)
	if err != nil {
		return nil, err
	}

	chain := &Chain{
		shardID:   lp.ShardID,
		epochID:   0,
		s:         s,
		cfg:       cfg,
		curHeader: &block.Header{},
	}

	genesisBlock, err := chain.initWithGenesisBlock()
	if err != nil {
		return nil, fmt.Errorf("create genesis block err: %w", err)
	}

	chain.curHeader = genesisBlock.Header

	return chain, nil
}

func (c *Chain) GetCurHeader() *block.Header {
	return c.curHeader
}

// GenerateBlock reads the current storage and tries to generate a normal block to handle the transactions.
// It will not affect the Chain.
func (c *Chain) GenerateBlock(ctx context.Context, miner account.Address, txs []transaction.Transaction) (*block.Block, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	bf, err := bloom.NewFilter(c.cfg.BloomFilterCfg)
	if err != nil {
		return nil, fmt.Errorf("new bloom filter err: %w", err)
	}

	for _, tx := range txs {
		txHash, err := utils.CalcHash(&tx)
		if err != nil {
			return nil, fmt.Errorf("calc hash err: %w", err)
		}

		bf.Add(txHash)
	}

	stateRoot, err := c.previewTrieUpdatedByTxs(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("preview updated trie by txs err: %w", err)
	}

	parentHeader, err := c.curHeader.Encode()
	if err != nil {
		return nil, fmt.Errorf("create parent header err: %w", err)
	}

	txRoot, err := c.getTxMerkleRoot(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("get tx trie stateRoot err: %w", err)
	}

	header := &block.Header{
		ParentBlockHash: parentHeader,
		StateRoot:       stateRoot,
		Number:          c.curHeader.Number + 1,
		Miner:           miner,
		CreateTime:      time.Now(),

		TxHeaderOpt: block.TxHeaderOpt{TxRoot: txRoot, Bloom: *bf},
	}

	return block.NewBlock(header, block.Body{TxList: txs}), nil
}

// GenerateMigrationBlock generates a block for account migration, by the given accounts and their states.
func (c *Chain) GenerateMigrationBlock(ctx context.Context, miner account.Address, accounts []account.Account, states []account.State) (*block.Block, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	stateRoot, err := c.previewTrieUpdatedByMigration(ctx, accounts, states)
	if err != nil {
		return nil, fmt.Errorf("preview updated trie by migration err: %w", err)
	}

	parentHeader, err := c.curHeader.Encode()
	if err != nil {
		return nil, fmt.Errorf("create parent header err: %w", err)
	}
	// calculate the merkle root of accounts & states.
	msRoot, err := c.getMigratedStateMerkleRoot(ctx, accounts, states)
	if err != nil {
		return nil, fmt.Errorf("get account state root err: %w", err)
	}

	header := &block.Header{
		ParentBlockHash: parentHeader,
		StateRoot:       stateRoot,
		Number:          c.curHeader.Number + 1,
		Miner:           miner,
		CreateTime:      time.Now(),

		MigrationHeaderOpt: block.MigrationHeaderOpt{MigratedAccountsRoot: msRoot},
	}

	return block.NewMigrationBlock(header, block.MigrationOpt{MigratedAccounts: accounts, MigratedStates: states}), nil
}

// AddBlock adds the given block into storage.
// It will modify the Chain.
// TODO(G Ye): AddBlock should be atomic.
func (c *Chain) AddBlock(ctx context.Context, b *block.Block) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	var (
		err                              error
		blockHash, blockByte, headerByte []byte
	)
	if blockHash, err = utils.CalcHash(b); err != nil {
		return fmt.Errorf("calc hash err: %w", err)
	}

	if blockByte, err = b.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}

	if headerByte, err = b.Header.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}

	// Update trie in db.
	if _, err = c.updateTrieByTxs(ctx, b); err != nil {
		return fmt.Errorf("update trie err: %w", err)
	}

	// add to storage
	err = c.s.BlockStorage.AddBlock(ctx, blockHash, blockByte, headerByte)
	if err != nil {
		return fmt.Errorf("failed to add block to storage: %w", err)
	}

	// update the current header
	c.curHeader = b.Header

	return nil
}

// GetAccountStates returns the shard-locations of all accounts by reading the MPT in the chain.
// It calls getAccountStates with a mutex.
func (c *Chain) GetAccountStates(ctx context.Context, accounts []account.Account) ([]*account.State, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	states, err := c.getAccountStates(ctx, accounts)
	if err != nil {
		return nil, fmt.Errorf("get account states err: %w", err)
	}

	return states, nil
}

// ValidateBlock validates blocks according to c's config.
// Note that, this function only validate block structure, but will not assert whether a block is valid to be added in this chain.
func (c *Chain) ValidateBlock(ctx context.Context, b *block.Block) error {
	// This block is a transaction block.
	if b.Header.TxRoot != nil {
		txRoot, err := c.getTxMerkleRoot(ctx, b.TxList)
		if err != nil {
			return fmt.Errorf("get tx trie stateRoot err: %w", err)
		}

		if !bytes.Equal(txRoot, b.Header.TxRoot) {
			return fmt.Errorf("tx root mismatch")
		}

		return nil
	}

	// This block is a migration block.
	mRoot, err := c.getMigratedStateMerkleRoot(ctx, b.MigratedAccounts, b.MigratedStates)
	if err != nil {
		return fmt.Errorf("get migrated account state merkle root err: %w", err)
	}

	if !bytes.Equal(mRoot, b.Header.MigratedAccountsRoot) {
		return fmt.Errorf("migration root mismatch")
	}

	return nil
}

func (c *Chain) GetShardID() int64 {
	return c.shardID
}

func (c *Chain) GetEpochID() int64 {
	return c.epochID
}

func (c *Chain) UpdateEpoch(epoch int64) {
	c.epochID = epoch
}

// Close closes the blockchain.
func (c *Chain) Close() error {
	err := c.s.BlockStorage.Close()
	if err != nil {
		return fmt.Errorf("close block storage err: %w", err)
	}

	err = c.s.TrieStorage.Close()
	if err != nil {
		return fmt.Errorf("close trie storage err: %w", err)
	}

	return nil
}

func (c *Chain) initWithGenesisBlock() (*block.Block, error) {
	genesisMiner := account.Address{}

	ctx := context.Background()

	b, err := c.GenerateBlock(ctx, genesisMiner, []transaction.Transaction{})
	if err != nil {
		return nil, fmt.Errorf("generate block err: %w", err)
	}

	if err = c.AddBlock(ctx, b); err != nil {
		return nil, fmt.Errorf("failed to add block to storage: %w", err)
	}

	return b, nil
}

func (c *Chain) previewTrieUpdatedByTxs(ctx context.Context, txs []transaction.Transaction) ([]byte, error) {
	keys, values, err := c.calculateAccountsAndStatesBytes(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
	}

	root, err := c.s.TrieStorage.MAddAccountStatesPreview(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("preview updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) previewTrieUpdatedByMigration(ctx context.Context, accounts []account.Account, states []account.State) ([]byte, error) {
	keyBytes, valBytes, err := c.getMigrationAccountBytes(accounts, states)
	if err != nil {
		return nil, fmt.Errorf("get migration account bytes err: %w", err)
	}

	root, err := c.s.TrieStorage.MAddAccountStatesPreview(ctx, keyBytes, valBytes)
	if err != nil {
		return nil, fmt.Errorf("preview updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) updateTrieByTxs(ctx context.Context, b *block.Block) ([]byte, error) {
	var keys, values [][]byte

	if b.Header.TxRoot != nil { // transaction block
		var err error

		keys, values, err = c.calculateAccountsAndStatesBytes(ctx, b.TxList)
		if err != nil {
			return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
		}
	} else { // migration block
		var err error

		keys, values, err = c.getMigrationAccountBytes(b.MigratedAccounts, b.MigratedStates)
		if err != nil {
			return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
		}
	}

	root, err := c.s.TrieStorage.MAddAccountStatesAndCommit(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("commit updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) calculateAccountsAndStatesBytes(ctx context.Context, txs []transaction.Transaction) ([][]byte, [][]byte, error) {
	accountStates := make(map[account.Account]*account.State, len(txs)*2)
	for _, tx := range txs {
		accountStates[tx.Sender] = nil
		accountStates[tx.Recipient] = nil

		// If this transaction is a broker tx, fetch the broker state
		if len(tx.BOriginalHash) > 0 {
			accountStates[tx.Broker] = nil
		}
	}

	accountList := maps.Keys(accountStates)

	originalStates, err := c.getAccountStates(ctx, accountList)
	if err != nil {
		return nil, nil, fmt.Errorf("get account states err: %w", err)
	}

	for i, a := range accountList {
		accountStates[a] = originalStates[i]
	}

	// update in map
	for _, tx := range txs {
		c.updateStateMapByTx(accountStates, tx)
	}

	// pack state list
	retAccountByteList, stateByteList := make([][]byte, 0, len(accountStates)), make([][]byte, 0, len(accountStates))

	for k, v := range accountStates {
		if v == nil { // this account is not in the shard
			continue
		}

		var kByte, vByte []byte

		if kByte, err = k.Encode(); err != nil {
			return nil, nil, fmt.Errorf("encode account from map err: %w", err)
		}

		if vByte, err = v.Encode(); err != nil {
			return nil, nil, fmt.Errorf("encode state from map err: %w", err)
		}

		retAccountByteList = append(retAccountByteList, kByte)
		stateByteList = append(stateByteList, vByte)
	}

	return retAccountByteList, stateByteList, nil
}

func (c *Chain) getMigrationAccountBytes(accounts []account.Account, states []account.State) ([][]byte, [][]byte, error) {
	var err error

	keyBytes := make([][]byte, len(accounts))

	valBytes := make([][]byte, len(accounts))
	for i, acc := range accounts {
		keyBytes[i], err = acc.Encode()
		if err != nil {
			return nil, nil, fmt.Errorf("encode account: %w", err)
		}

		valBytes[i], err = states[i].Encode()
		if err != nil {
			return nil, nil, fmt.Errorf("encode state err: %w", err)
		}
	}

	return keyBytes, valBytes, nil
}

func (c *Chain) updateStateMapByTx(accountStates map[account.Account]*account.State, tx transaction.Transaction) {
	if len(tx.ROriginalHash) != 0 { // relay transaction
		c.updateStateMapByRelayTx(accountStates, tx)
		return
	}

	if len(tx.BOriginalHash) != 0 { // broker transaction
		c.modifyStateMapByBrokerTx(accountStates, tx)
		return
	}

	c.modifyStateMapByNormalTx(accountStates, tx)
}

func (c *Chain) updateStateMapByRelayTx(accountStates map[account.Account]*account.State, tx transaction.Transaction) {
	switch tx.RelayStage {
	case transaction.Relay1Tx:
		// For a relay1 transaction, debit the sender's balance.
		senderState := accountStates[tx.Sender]
		if senderState == nil || senderState.ShardLocation != c.shardID {
			// Sender is not in this shard, skip.
			return
		}

		if err := senderState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of sender is not enough", "sender", tx.Sender, "value", tx.Value)
		} else if err != nil {
			slog.Error("debit error", "err", err)
		}

	case transaction.Relay2Tx:
		// For a relay2 transaction credit the recipient's balance.
		recipientState := accountStates[tx.Recipient]
		if recipientState == nil || recipientState.ShardLocation != c.shardID {
			return
		}

		recipientState.Credit(tx.Value)

	default:
		slog.Error("unexpected relay stage in updateStateMapByRelayTx", "stage", tx.RelayStage)
	}
}

func (c *Chain) modifyStateMapByBrokerTx(accountStates map[account.Account]*account.State, tx transaction.Transaction) {
	switch tx.BrokerStage {
	case transaction.Sigma1BrokerStage:
		// For a broker1 transaction, debit the sender's balance and credit the broker's balance.
		senderState := accountStates[tx.Sender]

		brokerState := accountStates[tx.Broker]
		if senderState == nil || senderState.ShardLocation != c.shardID {
			slog.Error("handle broker1 tx error", "err", "the sender is not in this shard", "sender", tx.Sender, "shard", c.shardID)
			return
		}

		if err := senderState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of sender is not enough", "sender", tx.Sender, "value", tx.Value)
		} else if err != nil {
			slog.Error("debit error", "err", err)
		} else {
			brokerState.Credit(tx.Value)
		}
	case transaction.Sigma2BrokerStage:
		// For a broker2 transaction, debit the broker's balance and credit the recipient's balance.
		recipientState := accountStates[tx.Recipient]

		brokerState := accountStates[tx.Broker]
		if recipientState == nil || recipientState.ShardLocation != c.shardID {
			slog.Error("handle broker2 tx error", "err", "the recipient is not in this shard", "recipient", tx.Recipient, "shard", c.shardID)
			return
		}

		if err := brokerState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of broker is not enough", "sender", tx.Sender, "value", tx.Value)
		} else if err != nil {
			slog.Error("debit error", "err", err)
		} else {
			recipientState.Credit(tx.Value)
		}
	default:
		slog.Error("unexpected broker stage in modifyStateMapByBrokerTx", "stage", tx.BrokerStage)
	}
}

func (c *Chain) modifyStateMapByNormalTx(accountStates map[account.Account]*account.State, tx transaction.Transaction) {
	senderState := accountStates[tx.Sender]
	recipientState := accountStates[tx.Recipient]

	// Modify senderState
	if senderState != nil && senderState.ShardLocation != c.shardID {
		if err := senderState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of sender is not enough", "sender", tx.Sender, "value", tx.Value)
		} else if err != nil {
			slog.Error("debit error", "err", err)
		}
	}

	// Modify recipientState
	if recipientState != nil && recipientState.ShardLocation != c.shardID {
		recipientState.Credit(tx.Value)
	}
}

func (c *Chain) getTxMerkleRoot(ctx context.Context, txs []transaction.Transaction) ([]byte, error) {
	var err error

	keyBytes := make([][]byte, len(txs))

	valBytes := make([][]byte, len(txs))

	for i, tx := range txs {
		keyBytes[i], err = utils.CalcHash(&tx)
		if err != nil {
			return nil, fmt.Errorf("hash err: %w", err)
		}

		valBytes[i], err = tx.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode tx err: %w", err)
		}
	}

	root, err := c.s.TrieStorage.GenerateRootByGivenBytes(ctx, keyBytes, valBytes)
	if err != nil {
		return nil, fmt.Errorf("generate root err: %w", err)
	}

	return root, nil
}

func (c *Chain) getMigratedStateMerkleRoot(ctx context.Context, accounts []account.Account, states []account.State) ([]byte, error) {
	keyBytes, valBytes, err := c.getMigrationAccountBytes(accounts, states)
	if err != nil {
		return nil, fmt.Errorf("get migrated state merkle root err: %w", err)
	}

	root, err := c.s.TrieStorage.GenerateRootByGivenBytes(ctx, keyBytes, valBytes)
	if err != nil {
		return nil, fmt.Errorf("generate root err: %w", err)
	}

	return root, nil
}

// getAccountStates get the states of accounts from the state trie.
// Note that, if the node is not existed in this state-trie, return a default state of this account.
func (c *Chain) getAccountStates(ctx context.Context, accounts []account.Account) ([]*account.State, error) {
	accountByteList := make([][]byte, len(accounts))
	for i, addr := range accounts {
		aByte, err := addr.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode addr err: %w", err)
		}

		accountByteList[i] = aByte
	}

	stateByteList, err := c.s.TrieStorage.MGetAccountStates(ctx, accountByteList)
	if err != nil {
		return nil, fmt.Errorf("get account states from trie err: %w", err)
	}

	states := make([]*account.State, len(accounts))

	for i, stateByte := range stateByteList {
		if stateByte == nil {
			// set the default state
			states[i] = account.NewState(accounts[i], partition.DefaultAccountLoc(accounts[i].Addr, c.cfg.ShardNum))
			continue
		}

		if states[i], err = account.DecodeState(stateByte); err != nil {
			return nil, fmt.Errorf("decode state err: %w", err)
		}
	}

	return states, nil
}
