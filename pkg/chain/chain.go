package chain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/bloom"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage"
)

const blocksFetchLimit = 100

// Chain describes a blockchain.
type Chain struct {
	s         *storage.Storage // the storage for both block-storage, trie-storage and geth's state db.
	curHeader block.Header     // the current header in this blockchain.
	shardID   int64
	epochID   int64

	cfg        config.BlockchainCfg
	vmChainCfg *params.ChainConfig

	mux sync.Mutex
}

// NewChain creates a new blockchain data structure with given components.
func NewChain(cfg config.BlockchainCfg, lp config.LocalParams) (*Chain, error) {
	s, err := storage.NewStorage(cfg.StorageCfg, lp)
	if err != nil {
		return nil, err
	}

	vmChainCfg := *params.MainnetChainConfig
	vmChainCfg.ChainID = big.NewInt(cfg.ChainID)

	chain := &Chain{
		s:         s,
		curHeader: block.Header{},
		epochID:   0,
		shardID:   lp.ShardID,

		cfg:        cfg,
		vmChainCfg: &vmChainCfg,
	}

	genesisBlock, err := chain.initWithGenesisBlock()
	if err != nil {
		return nil, fmt.Errorf("create genesis block err: %w", err)
	}

	chain.curHeader = genesisBlock.Header

	return chain, nil
}

func (c *Chain) GetCurHeader() block.Header {
	return c.curHeader
}

// GenerateBlock reads the current storage and tries to generate a block to handle the body and the migrationOpt.
// It will not affect the Chain.
func (c *Chain) GenerateBlock(
	ctx context.Context,
	miner account.Address,
	blockType uint8,
	body block.Body,
	mOpt block.MigrationOpt,
) (*block.Block, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	parentHeader, err := c.curHeader.Hash()
	if err != nil {
		return nil, fmt.Errorf("create parent header err: %w", err)
	}

	// Calculate the TxHeaderOpt.
	tOpt, err := c.calcTxHeaderOpt(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("calc tx header opt err: %w", err)
	}

	// Calculate the MigrationTxOpt.
	mHeaderOpt, err := c.calcMigrationHeaderOpt(ctx, mOpt)
	if err != nil {
		return nil, fmt.Errorf("get account state root err: %w", err)
	}

	header := block.Header{
		ParentBlockHash: parentHeader,
		Number:          c.curHeader.Number + 1,
		Miner:           miner,
		Type:            blockType,
		CreateTime:      time.Now(),

		TxHeaderOpt:        *tOpt,
		MigrationHeaderOpt: *mHeaderOpt,
	}

	b := block.NewBlock(header, body, mOpt)

	// Calculate and set the state root in the block.
	stateRoot, err := c.previewStateRootByBlock(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("preview updated trie by txs err: %w", err)
	}

	b.StateRoot = stateRoot

	return b, nil
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
	if blockHash, err = b.Hash(); err != nil {
		return fmt.Errorf("calc header hash err: %w", err)
	}

	if blockByte, err = b.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}

	if headerByte, err = b.Header.Encode(); err != nil {
		return fmt.Errorf("encode block header err: %w", err)
	}

	// Update trie in db.
	if _, err = c.updateTrieByBlock(ctx, b); err != nil {
		return fmt.Errorf("update trie err: %w", err)
	}

	// Add to storage.
	err = c.s.BlockStorage.AddBlock(ctx, blockHash, blockByte, headerByte)
	if err != nil {
		return fmt.Errorf("failed to add block to storage: %w", err)
	}

	// Update the current header
	c.curHeader = b.Header

	slog.InfoContext(ctx, "block is generated",
		"shard ID", c.GetShardID(), "block height", b.Number, "block create time", b.CreateTime)

	return nil
}

// GetAccountStates returns the shard-locations of all accounts by reading the MPT in the chain.
// It calls getAccountStates with a mutex.
func (c *Chain) GetAccountStates(ctx context.Context, accounts []account.Address) ([]*account.State, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	states, err := c.getAccountStates(ctx, accounts)
	if err != nil {
		return nil, fmt.Errorf("get account states err: %w", err)
	}

	return states, nil
}

// GetAccountLocationsInTxs gets the locations of the accounts in the given transaction list.
func (c *Chain) GetAccountLocationsInTxs(
	ctx context.Context,
	txs []transaction.Transaction,
) (map[account.Address]int64, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	// get all locations of accounts.
	accountLocations := make(map[account.Address]int64)
	for _, tx := range txs {
		accountLocations[tx.Sender] = -1
		accountLocations[tx.Recipient] = -1
	}

	requestAccounts := maps.Keys(accountLocations)

	states, err := c.getAccountStates(ctx, requestAccounts)
	if err != nil {
		return nil, fmt.Errorf("GetAccountStates failed: %w", err)
	}

	for i, requestAccount := range requestAccounts {
		accountLocations[requestAccount] = int64(states[i].ShardLocation)
	}

	return accountLocations, nil
}

// ValidateBlock validates blocks according to the chain's config.
func (c *Chain) ValidateBlock(ctx context.Context, b *block.Block) error {
	// Validate the transaction part.
	tH, err := c.calcTxHeaderOpt(ctx, b.Body)
	if err != nil {
		return fmt.Errorf("get tx trie stateRoot err: %w", err)
	}

	if !bytes.Equal(tH.TxRoot, b.TxRoot) {
		return fmt.Errorf("tx root mismatch")
	}

	if !tH.Bloom.Equal(b.Bloom) {
		return fmt.Errorf("bloom mismatch")
	}

	// Validate the migration part.
	mH, err := c.calcMigrationHeaderOpt(ctx, b.MigrationOpt)
	if err != nil {
		return fmt.Errorf("get migrated account state Merkle root err: %w", err)
	}

	if !bytes.Equal(mH.MigratedAccountsRoot, b.MigratedAccountsRoot) {
		return fmt.Errorf("migration root mismatch")
	}

	return nil
}

// GetBlocksAfterHeight gets blocks those heights are larger than the given beginHeight.
// The heights of the returning blocks is in [beginHeight, curHeight].
func (c *Chain) GetBlocksAfterHeight(ctx context.Context, beginHeight int64) ([]block.Block, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if beginHeight <= 0 {
		return nil, fmt.Errorf("beginHeight must be > 0")
	}

	fetchedBlocksCnt := int64(c.curHeader.Number) - beginHeight + 1
	if fetchedBlocksCnt > blocksFetchLimit {
		return nil, fmt.Errorf("too many blocks (%d) to return", fetchedBlocksCnt)
	}

	if fetchedBlocksCnt < 0 {
		fetchedBlocksCnt = 0
	}

	bHash, err := c.curHeader.Hash()
	if err != nil {
		return nil, fmt.Errorf("get header hash err: %w", err)
	}

	var (
		bByte []byte
		b     *block.Block
	)

	blocks := make([]block.Block, fetchedBlocksCnt)

	for i := fetchedBlocksCnt - 1; i >= 0; i-- {
		bByte, err = c.s.BlockStorage.GetBlockByHash(ctx, bHash)
		if err != nil {
			return nil, fmt.Errorf("get block err: %w", err)
		}

		b, err = block.DecodeBlock(bByte)
		if err != nil {
			return nil, fmt.Errorf("decode block err: %w", err)
		}

		slog.InfoContext(ctx, "block is fetched from storage", "height", b.Number)

		blocks[i] = *b
		// Set the hash to the parent hash
		bHash = b.ParentBlockHash
	}

	return blocks, nil
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

	err = c.s.LocStorage.Close()
	if err != nil {
		return fmt.Errorf("close trie storage err: %w", err)
	}

	return nil
}

func (c *Chain) initWithGenesisBlock() (*block.Block, error) {
	genesisMiner := account.Address{}

	ctx := context.Background()

	b, err := c.GenerateBlock(ctx, genesisMiner, block.TxBlockType, block.Body{}, block.MigrationOpt{})
	if err != nil {
		return nil, fmt.Errorf("generate block err: %w", err)
	}

	if err = c.AddBlock(ctx, b); err != nil {
		return nil, fmt.Errorf("failed to add block to storage: %w", err)
	}

	return b, nil
}

func (c *Chain) previewStateRootByBlock(ctx context.Context, b *block.Block) ([]byte, error) {
	keys, values, err := c.calcModifiedAccountBytes(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
	}

	root, err := c.s.LocStorage.MAddKeyValuesPreview(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("preview updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) updateTrieByBlock(ctx context.Context, b *block.Block) ([]byte, error) {
	keys, values, err := c.calcModifiedAccountBytes(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("calculate the modified accounts bytes by the given block err: %w", err)
	}

	root, err := c.s.LocStorage.MAddKeyValuesAndCommit(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("commit updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) calcModifiedAccountBytes(ctx context.Context, b *block.Block) ([][]byte, [][]byte, error) {
	txs := b.TxList
	migratedAccounts := b.MigratedAccounts
	migratedStates := b.MigratedStates

	accountStates := make(map[account.Address]*account.State, len(txs)*2)
	for _, tx := range txs {
		accountStates[tx.Sender] = nil
		accountStates[tx.Recipient] = nil

		// If this transaction is a broker tx, fetch the broker state
		if tx.TxType() == transaction.BrokerTxType {
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

	// Set the state of accounts by the given MigrationOpt.
	for i, ma := range migratedAccounts {
		accountStates[ma] = &migratedStates[i]
	}

	// Update in map.
	for _, tx := range txs {
		c.executeTx(accountStates, tx)
	}

	// Pack state list.
	keys, vals := make([][]byte, 0, len(accountStates)), make([][]byte, 0, len(accountStates))

	for k, v := range accountStates {
		if v == nil { // this account is not in the shard
			continue
		}

		var kByte, vByte []byte

		kByte = k[:]

		if vByte, err = v.Encode(); err != nil {
			return nil, nil, fmt.Errorf("encode state from map err: %w", err)
		}

		keys = append(keys, kByte)
		vals = append(vals, vByte)
	}

	return keys, vals, nil
}

func (c *Chain) getMigrationAccountBytes(accList []account.Address, sList []account.State) ([][]byte, [][]byte, error) {
	var err error

	keyBytes, valBytes := make([][]byte, len(accList)), make([][]byte, len(accList))
	for i, acc := range accList {
		keyBytes[i] = acc[:]

		valBytes[i], err = sList[i].Encode()
		if err != nil {
			return nil, nil, fmt.Errorf("encode state err: %w", err)
		}
	}

	return keyBytes, valBytes, nil
}

func (c *Chain) executeTx(accountStates map[account.Address]*account.State, tx transaction.Transaction) {
	switch tx.TxType() {
	case transaction.NormalTxType:
		c.executeNormalTx(accountStates, tx)
	case transaction.RelayTxType:
		c.executeRelayTx(accountStates, tx)
	case transaction.BrokerTxType:
		c.executeBrokerTx(accountStates, tx)
	}
}

func (c *Chain) executeRelayTx(accountStates map[account.Address]*account.State, tx transaction.Transaction) {
	switch tx.RelayStage {
	case transaction.Relay1Tx:
		// For a relay1 transaction, debit the sender's balance.
		senderState := accountStates[tx.Sender]
		if senderState == nil || senderState.ShardLocation != uint64(c.shardID) {
			// Sender is not in this shard, skip.
			return
		}

		if err := senderState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of sender is not enough", "sender", tx.Sender, "value", tx.Value)
		} else if err != nil {
			slog.Error("debit error", "err", err)
		}

		senderState.Nonce = tx.Nonce

	case transaction.Relay2Tx:
		// For a relay2 transaction credit the recipient's balance.
		recipientState := accountStates[tx.Recipient]
		if recipientState == nil || recipientState.ShardLocation != uint64(c.shardID) {
			return
		}

		recipientState.Credit(tx.Value)

	default:
		slog.Error("unexpected relay stage in executeRelayTx", "stage", tx.RelayStage)
	}
}

func (c *Chain) executeBrokerTx(accountStates map[account.Address]*account.State, tx transaction.Transaction) {
	switch tx.BrokerStage {
	case transaction.Sigma1BrokerStage:
		// For a broker1 transaction, debit the sender's balance and credit the broker's balance.
		senderState := accountStates[tx.Sender]

		brokerState := accountStates[tx.Broker]

		if senderState == nil || senderState.ShardLocation != uint64(c.shardID) {
			slog.Error("handle broker1 tx error", "err", "the sender is not in this shard")
			return
		}

		if err := senderState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of sender is not enough", "sender", tx.Sender, "value", tx.Value)
		} else if err != nil {
			slog.Error("debit error", "err", err)
		} else {
			senderState.Nonce = tx.Nonce
			brokerState.Credit(tx.Value)
		}
	case transaction.Sigma2BrokerStage:
		// For a broker2 transaction, debit the broker's balance and credit the recipient's balance.
		recipientState := accountStates[tx.Recipient]

		brokerState := accountStates[tx.Broker]

		if recipientState == nil || recipientState.ShardLocation != uint64(c.shardID) {
			slog.Error("handle broker2 tx error", "err", "the recipient is not in this shard")
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
		slog.Error("unexpected broker stage in executeBrokerTx", "stage", tx.BrokerStage)
	}
}

func (c *Chain) executeNormalTx(accountStates map[account.Address]*account.State, tx transaction.Transaction) {
	senderState := accountStates[tx.Sender]
	recipientState := accountStates[tx.Recipient]

	// Modify senderState
	if senderState != nil && senderState.ShardLocation == uint64(c.shardID) {
		if err := senderState.Debit(tx.Value); errors.Is(err, account.ErrNotEnoughBalance) {
			slog.Warn("the balance of sender is not enough", "sender", tx.Sender, "value", tx.Value)
			return
		} else if err != nil {
			slog.Error("debit error", "err", err)
			return
		}

		senderState.Nonce = tx.Nonce
	}

	// Modify recipientState
	if recipientState != nil && recipientState.ShardLocation == uint64(c.shardID) {
		recipientState.Credit(tx.Value)
	}
}

func (c *Chain) calcTxHeaderOpt(ctx context.Context, body block.Body) (*block.TxHeaderOpt, error) {
	if len(body.TxList) == 0 {
		return &block.TxHeaderOpt{}, nil
	}
	// Calculate the bloom filter.
	bf, err := bloom.NewFilter(c.cfg.BloomFilterCfg)
	if err != nil {
		return nil, fmt.Errorf("new bloom filter err: %w", err)
	}

	// Calculate the transaction root.
	keyBytes, valBytes := make([][]byte, len(body.TxList)), make([][]byte, len(body.TxList))

	for i, tx := range body.TxList {
		keyBytes[i], err = tx.Hash()
		if err != nil {
			return nil, fmt.Errorf("hash err: %w", err)
		}

		valBytes[i], err = tx.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode tx err: %w", err)
		}
	}

	bf.Add(keyBytes...)

	root, err := c.s.LocStorage.GenerateRootByGivenBytes(ctx, keyBytes, valBytes)
	if err != nil {
		return nil, fmt.Errorf("generate root err: %w", err)
	}

	return &block.TxHeaderOpt{TxRoot: root, Bloom: *bf}, nil
}

func (c *Chain) calcMigrationHeaderOpt(ctx context.Context, opt block.MigrationOpt) (*block.MigrationHeaderOpt, error) {
	if len(opt.MigratedAccounts) == 0 {
		return &block.MigrationHeaderOpt{}, nil
	}

	keyBytes, valBytes, err := c.getMigrationAccountBytes(opt.MigratedAccounts, opt.MigratedStates)
	if err != nil {
		return nil, fmt.Errorf("get migrated state Merkle root err: %w", err)
	}

	root, err := c.s.LocStorage.GenerateRootByGivenBytes(ctx, keyBytes, valBytes)
	if err != nil {
		return nil, fmt.Errorf("generate root err: %w", err)
	}

	return &block.MigrationHeaderOpt{MigratedAccountsRoot: root}, nil
}

// getAccountStates get the states of accounts from the state trie.
// Note that, if the node is not existed in this state-trie, return a default state of this account.
func (c *Chain) getAccountStates(ctx context.Context, addresses []account.Address) ([]*account.State, error) {
	accountByteList := make([][]byte, len(addresses))
	for i, addr := range addresses {
		accountByteList[i] = addr[:]
	}

	stateByteList, err := c.s.LocStorage.MGetValsByKeys(ctx, accountByteList)
	if err != nil {
		return nil, fmt.Errorf("get account states from trie err: %w", err)
	}

	states := make([]*account.State, len(addresses))

	for i, stateByte := range stateByteList {
		if stateByte == nil {
			// set the default state
			states[i] = account.NewState(addresses[i], partition.DefaultAccountLoc(addresses[i], c.cfg.ShardNum))
			continue
		}

		if states[i], err = account.DecodeState(stateByte); err != nil {
			return nil, fmt.Errorf("decode state err: %w", err)
		}
	}

	return states, nil
}
