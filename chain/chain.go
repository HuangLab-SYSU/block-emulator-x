package chain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
	"github.com/HuangLab-SYSU/block-emulator/core/block"
	"github.com/HuangLab-SYSU/block-emulator/core/bloom"
	"github.com/HuangLab-SYSU/block-emulator/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/storage"
	"github.com/HuangLab-SYSU/block-emulator/utils"
)

type Chain struct {
	s         *storage.Storage
	curHeader *block.Header
	cfg       config.BlockchainCfg

	mux sync.Mutex
}

// NewChain creates a new blockchain data structure with given components.
func NewChain(cfg config.BlockchainCfg) (*Chain, error) {
	s, err := storage.NewStorage(cfg.StorageCfg)
	if err != nil {
		return nil, err
	}
	chain := &Chain{
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
func (c *Chain) AddBlock(ctx context.Context, b *block.Block) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	// TODO: AddBlock should be atomic
	var err error
	var blockHash, blockByte, headerByte []byte
	// validate block
	if blockHash, err = utils.CalcHash(b); err != nil {
		return fmt.Errorf("calc hash err: %w", err)
	}
	if blockByte, err = b.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}
	if blockHash, err = b.Header.Encode(); err != nil {
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
	err = c.s.BlockStorage.Close()
	if err != nil {
		return fmt.Errorf("close block storage err: %w", err)
	}
	return nil
}

func (c *Chain) initWithGenesisBlock() (*block.Block, error) {
	genesisMiner := account.Address{}
	var b *block.Block
	var err error
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
	account2State := make(map[account.Account]*account.State, len(txs)*2)
	for _, tx := range txs {
		account2State[tx.Sender] = nil
		account2State[tx.Recipient] = nil
	}

	accountByteList := make([][]byte, 0, len(account2State))
	for a := range account2State {
		encodedAccount, _ := a.Encode()
		accountByteList = append(accountByteList, encodedAccount)
	}

	statesList, err := c.s.TrieStorage.MGetAccountStates(ctx, accountByteList)
	if err != nil {
		return nil, nil, fmt.Errorf("get addr state err: %w", err)
	}
	for i, accountByte := range accountByteList {
		a, _ := account.DecodeAccount(accountByte)
		s, err := account.DecodeState(statesList[i])
		if err != nil {
			return nil, nil, fmt.Errorf("decode state err: %w", err)
		}
		account2State[*a] = s
	}

	// update in map
	for _, tx := range txs {
		senderState := account2State[tx.Sender]
		recipientState := account2State[tx.Recipient]
		if senderState == nil || recipientState == nil {
			return nil, nil, fmt.Errorf("sender or recipient state is nil")
		}
		if senderState.Debit(tx.Value) != nil {
			// TODO: check whether to continue
			continue
		}
		recipientState.Credit(tx.Value)
	}

	// pack state list
	stateByteList := make([][]byte, len(account2State))
	for i, accountByte := range accountByteList {
		a, _ := account.DecodeAccount(accountByte)
		stateByteList[i], err = account2State[*a].Encode()
		if err != nil {
			return nil, nil, fmt.Errorf("encode account err: %w", err)
		}
	}
	return accountByteList, stateByteList, nil
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
