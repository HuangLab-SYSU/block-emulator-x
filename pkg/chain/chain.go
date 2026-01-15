package chain

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/bloom"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
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
		curHeader: block.Header{StateRoot: types.EmptyRootHash[:]},
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
	tOpt, err := c.calcTxHeaderOpt(body)
	if err != nil {
		return nil, fmt.Errorf("calc tx header opt err: %w", err)
	}

	// Calculate the MigrationTxOpt.
	mHeaderOpt, err := c.calcMigrationHeaderOpt(mOpt)
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
	stateRoot, locRoot, err := c.previewStateRootByBlock(ctx, b)
	if err != nil {
		return nil, fmt.Errorf("preview updated trie by txs err: %w", err)
	}

	b.StateRoot = stateRoot
	b.LocationRoot = locRoot

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

	// Update the location trie and vm trie db.
	if _, _, err = c.updateTrieByBlock(ctx, b); err != nil {
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

	return c.getAccountLocationsInTxs(ctx, txs)
}

// ValidateBlock validates blocks according to the chain's config.
func (c *Chain) ValidateBlock(_ context.Context, b *block.Block) error {
	// Validate the transaction part.
	tH, err := c.calcTxHeaderOpt(b.Body)
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
	mH, err := c.calcMigrationHeaderOpt(b.MigrationOpt)
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

func (c *Chain) previewStateRootByBlock(ctx context.Context, b *block.Block) ([]byte, []byte, error) {
	vme, accountBytes, locationBytes, err := c.calcStateModification(ctx, b)
	if err != nil {
		return nil, nil, fmt.Errorf("calc state modification err: %w", err)
	}

	// Preview the result but not commit to disk.
	stateRoot := vme.StateDB().IntermediateRoot(true)

	locRoot, err := c.s.LocStorage.MAddKeyValuesPreview(ctx, accountBytes, locationBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("preview updated accounts err: %w", err)
	}

	return stateRoot[:], locRoot, nil
}

func (c *Chain) updateTrieByBlock(ctx context.Context, b *block.Block) ([]byte, []byte, error) {
	vme, accountBytes, locationBytes, err := c.calcStateModification(ctx, b)
	if err != nil {
		return nil, nil, fmt.Errorf("calc state modification err: %w", err)
	}

	// Preview the result but not commit to disk.
	stateRoot, err := vme.Commit(getBlockCtxByBlock(b).BlockNumber.Uint64())
	if err != nil {
		return nil, nil, fmt.Errorf("vme commit err: %w", err)
	}

	locRoot, err := c.s.LocStorage.MAddKeyValuesAndCommit(ctx, accountBytes, locationBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("preview updated accounts err: %w", err)
	}

	return stateRoot[:], locRoot, nil
}

func (c *Chain) getAccountLocationsInTxs(
	ctx context.Context,
	txs []transaction.Transaction,
) (map[account.Address]int64, error) {
	// Get all locations of accounts.
	accountLocations := make(map[account.Address]int64)
	for _, tx := range txs {
		accountLocations[tx.Sender] = -1
		accountLocations[tx.Recipient] = -1
	}

	requestAccounts := maps.Keys(accountLocations)

	locations, err := c.getAccountLocations(ctx, requestAccounts)
	if err != nil {
		return nil, fmt.Errorf("GetAccountStates failed: %w", err)
	}

	for i, requestAccount := range requestAccounts {
		accountLocations[requestAccount] = locations[i]
	}

	return accountLocations, nil
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

func (c *Chain) txExecute(
	v *vm.Executor,
	bCtx gethvm.BlockContext,
	addrLoc map[account.Address]int64,
	tx transaction.Transaction,
) error {
	txCtx := gethvm.TxContext{
		Origin: common.Address(tx.Sender),
	}
	switch tx.TxType() {
	case transaction.NormalTxType:
		if err := normalTxExecute(v, tx); err != nil {
			return fmt.Errorf("execute normal tx failed: %w", err)
		}
	case transaction.RelayTxType:
		if err := relayTxExecute(v, addrLoc, c.shardID, tx); err != nil {
			return fmt.Errorf("execute relay tx failed: %w", err)
		}
	case transaction.BrokerTxType:
		if err := brokerTxExecute(v, addrLoc, c.shardID, tx); err != nil {
			return fmt.Errorf("execute broker tx failed: %w", err)
		}
	case transaction.CreateContractTxType:
		contractAddr, _, err := v.DeployContract(bCtx, txCtx, tx.Sender, tx.Data, tx.Value, tx.GasLimit)
		if err != nil {
			return fmt.Errorf("failed to deploy contract: %w", err)
		}

		slog.Info("deploy contract succeed", "contract addr", contractAddr)
	case transaction.CallContractTxType:
		ret, _, err := v.CallContract(bCtx, txCtx, tx.Sender, tx.Recipient, tx.Data, tx.Value, tx.GasLimit)
		if err != nil {
			return fmt.Errorf("failed to call contract: %w", err)
		}

		slog.Info("call contract succeed", "result", ret)
	default:
		return fmt.Errorf("unknown transaction type: %b", tx.TxType())
	}

	return nil
}

func (c *Chain) calcTxHeaderOpt(body block.Body) (*block.TxHeaderOpt, error) {
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

	root, err := utils.GenerateRootByGivenBytes(keyBytes, valBytes)
	if err != nil {
		return nil, fmt.Errorf("generate root err: %w", err)
	}

	return &block.TxHeaderOpt{TxRoot: root, Bloom: *bf}, nil
}

func (c *Chain) calcMigrationHeaderOpt(mOpt block.MigrationOpt) (*block.MigrationHeaderOpt, error) {
	if len(mOpt.MigratedAddrs) == 0 {
		return &block.MigrationHeaderOpt{}, nil
	}

	keyBytes, valBytes, err := c.getMigrationAccountBytes(mOpt.MigratedAddrs, mOpt.MigratedStates)
	if err != nil {
		return nil, fmt.Errorf("get migrated state Merkle root err: %w", err)
	}

	root, err := utils.GenerateRootByGivenBytes(keyBytes, valBytes)
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

	locations, err := c.getAccountLocations(ctx, addresses)
	if err != nil {
		return nil, fmt.Errorf("get account locations err: %w", err)
	}

	vme, err := c.getVMExecutor()
	if err != nil {
		return nil, fmt.Errorf("new vm executor err: %w", err)
	}

	states := make([]*account.State, len(addresses))

	for i, addr := range addresses {
		states[i] = readStateFromVMExecutor(addr, vme, uint64(locations[i]))
	}

	return states, nil
}

// getAccountLocations returns the locations of accounts.
// If the account does not exist, a default location will be returned.
func (c *Chain) getAccountLocations(ctx context.Context, addresses []account.Address) ([]int64, error) {
	accountByteList := make([][]byte, len(addresses))
	for i, addr := range addresses {
		accountByteList[i] = addr[:]
	}

	locBytes, err := c.s.LocStorage.MGetValsByKeys(ctx, accountByteList)
	if err != nil {
		return nil, fmt.Errorf("get account locations from trie err: %w", err)
	}

	locations := make([]int64, len(addresses))

	for i, locByte := range locBytes {
		if locByte == nil {
			// Set the default location.
			locations[i] = partition.DefaultAccountLoc(addresses[i], c.cfg.ShardNum)
			continue
		}

		var uintLoc uint64
		if err = gob.NewDecoder(bytes.NewReader(locByte)).Decode(&uintLoc); err != nil {
			return nil, fmt.Errorf("decode location err: %w", err)
		}

		locations[i] = int64(uintLoc)
	}

	return locations, nil
}

func (c *Chain) getVMExecutor() (*vm.Executor, error) {
	return vm.NewExecutor(c.s.VMTrieDB, common.Hash(c.curHeader.StateRoot), c.vmChainCfg)
}

func (c *Chain) calcStateModification(ctx context.Context, b *block.Block) (*vm.Executor, [][]byte, [][]byte, error) {
	accountLocMap, err := c.getAccountLocationsInTxs(ctx, b.TxList)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get account locations err: %w", err)
	}

	vme, err := c.getVMExecutor()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get vm executor err: %w", err)
	}

	// Handle the transactions in Body.
	bCtx := getBlockCtxByBlock(b)
	for _, tx := range b.TxList {
		if err = c.txExecute(vme, bCtx, accountLocMap, tx); err != nil {
			return nil, nil, nil, fmt.Errorf("execute tx err: %w", err)
		}
	}

	// Handle the migrated accounts in MigrationOpt.
	accountBytes := make([][]byte, len(b.MigratedAddrs))

	locationBytes := make([][]byte, len(b.MigratedAddrs))
	for i, acc := range b.MigratedAddrs {
		state := b.MigratedStates[i]
		if err = setMigratedStates2VMTrie(acc, state, vme); err != nil {
			return nil, nil, nil, fmt.Errorf("set migrated state err: %w", err)
		}

		accountBytes[i] = acc[:]

		var buf bytes.Buffer
		if err = gob.NewEncoder(&buf).Encode(state.ShardLocation); err != nil {
			return nil, nil, nil, fmt.Errorf("encode state err: %w", err)
		}

		locationBytes[i] = buf.Bytes()
	}

	return vme, accountBytes, locationBytes, nil
}
