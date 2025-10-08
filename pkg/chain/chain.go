package chain

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/bloom"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/storage"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

type Chain struct {
	s         *storage.Storage
	curHeader *block.Header
	shardID   int64

	cfg config.BlockchainCfg

	mux sync.Mutex
}

// NewChain creates a new blockchain data structure with given components.
func NewChain(cfg config.BlockchainCfg, shardID int64) (*Chain, error) {
	if cfg.ShardNum <= 0 {
		return nil, fmt.Errorf("expected shard number >= 0, got %d", cfg.ShardNum)
	}

	if cfg.ShardNum <= shardID {
		return nil, fmt.Errorf("expected shard id < shard number, got %d", shardID)
	}

	s, err := storage.NewStorage(cfg.StorageCfg)
	if err != nil {
		return nil, err
	}

	chain := &Chain{
		shardID:   shardID,
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

// GenerateBlock reads the current storage and tries to generate a block.
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

	stateRoot, err := c.previewUpdatedTrie(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("preview updated trie err: %w", err)
	}

	parentHeader, err := c.curHeader.Encode()
	if err != nil {
		return nil, fmt.Errorf("create parent header err: %w", err)
	}

	txRoot, err := c.getTxTrieRoot(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("get tx trie stateRoot err: %w", err)
	}

	header := &block.Header{
		ParentBlockHash: parentHeader,
		StateRoot:       stateRoot,
		TxRoot:          txRoot,
		Bloom:           *bf,
		Number:          c.curHeader.Number + 1,
		Miner:           miner,
		CreateTime:      time.Now(),
	}

	return block.NewBlock(header, &block.Body{TxList: txs}), nil
}

// AddBlock adds the given block into storage.
// It will modify the Chain.
// TODO(Guang Ye): AddBlock should be atomic.
func (c *Chain) AddBlock(ctx context.Context, b *block.Block) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	var (
		err                              error
		blockHash, blockByte, headerByte []byte
	)
	// validate block

	if blockHash, err = utils.CalcHash(b); err != nil {
		return fmt.Errorf("calc hash err: %w", err)
	}

	if blockByte, err = b.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}

	if headerByte, err = b.Header.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}

	// update trie in db
	if _, err = c.updateTrie(ctx, b.Body.TxList); err != nil {
		return fmt.Errorf("update trie err: %w", err)
	}
	// add to storage
	err = c.s.BlockStorage.AddBlock(ctx, blockHash, blockByte, headerByte)
	if err != nil {
		return fmt.Errorf("failed to add block to storage: %w", err)
	}

	return nil
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

	var (
		b   *block.Block
		err error
	)

	ctx := context.Background()
	if b, err = c.GenerateBlock(ctx, genesisMiner, []transaction.Transaction{}); err != nil {
		return nil, fmt.Errorf("generate block err: %w", err)
	}

	if err = c.AddBlock(ctx, b); err != nil {
		return nil, fmt.Errorf("failed to add block to storage: %w", err)
	}

	return b, nil
}

func (c *Chain) previewUpdatedTrie(ctx context.Context, txs []transaction.Transaction) ([]byte, error) {
	keys, values, err := c.getUpdatedAccountsBytes(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
	}

	root, err := c.s.TrieStorage.MAddAccountStatesPreview(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("preview updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) updateTrie(ctx context.Context, txs []transaction.Transaction) ([]byte, error) {
	keys, values, err := c.getUpdatedAccountsBytes(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
	}

	root, err := c.s.TrieStorage.MAddAccountStatesAndCommit(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("commit updated accounts err: %w", err)
	}

	return root, nil
}

func (c *Chain) getUpdatedAccountsBytes(ctx context.Context, txs []transaction.Transaction) ([][]byte, [][]byte, error) {
	account2StateInShard := make(map[account.Account]*account.State, len(txs)*2)
	for _, tx := range txs {
		account2StateInShard[tx.Sender] = nil
		account2StateInShard[tx.Recipient] = nil
	}

	accountByteList := make([][]byte, 0, len(account2StateInShard))

	for a := range account2StateInShard {
		encodedAccount, _ := a.Encode()
		accountByteList = append(accountByteList, encodedAccount)
	}

	originStateByteList, err := c.s.TrieStorage.MGetAccountStates(ctx, accountByteList)
	if err != nil {
		return nil, nil, fmt.Errorf("get addr state err: %w", err)
	}

	for i, accountByte := range accountByteList {
		a, _ := account.DecodeAccount(accountByte)
		osb := originStateByteList[i]

		var s *account.State = nil

		if osb != nil {
			if s, err = account.DecodeState(osb); err != nil {
				return nil, nil, fmt.Errorf("decode state err: %w", err)
			}
		}

		// if it is a new account, init it.
		if s == nil {
			s = generateInitAccountState(*a, c.cfg.ShardNum)
		}
		// this account is not in the shard, skip it
		if !slices.Contains(s.ShardLocations, c.shardID) {
			continue
		}

		account2StateInShard[*a] = s
	}

	// update in map
	for _, tx := range txs {
		senderState := account2StateInShard[tx.Sender]
		recipientState := account2StateInShard[tx.Recipient]
		// if sender exists in this shard, try to debit it. otherwise, skip debit.
		// if the debit operation failed, skip this transaction.
		if senderState == nil || errors.Is(senderState.Debit(tx.Value), account.ErrNotEnoughBalance) {
			// TODO(Guang Ye): check whether to continue, or report error
			continue
		}
		// if recipient exists in this shard, credit it.
		if recipientState != nil {
			recipientState.Credit(tx.Value)
		}
	}

	// pack state list
	retAccountByteList := make([][]byte, 0, len(account2StateInShard))

	stateByteList := make([][]byte, 0, len(account2StateInShard))

	for k, v := range account2StateInShard {
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

func (c *Chain) getTxTrieRoot(ctx context.Context, txs []transaction.Transaction) ([]byte, error) {
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
