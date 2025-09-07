package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
	"github.com/HuangLab-SYSU/block-emulator/core/block"
	"github.com/HuangLab-SYSU/block-emulator/core/bloom"
	"github.com/HuangLab-SYSU/block-emulator/core/hash"
	"github.com/HuangLab-SYSU/block-emulator/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/storage"
)

const bloomFilterLen = 1 << 12

type Chain struct {
	curHeader *block.Header
	s         storage.Storage
	txPool    txpool.TxPool
	cfg       config.BlockchainCfg
}

// NewChain creates a new blockchain data structure with given components.
func NewChain(cfg config.BlockchainCfg, s storage.Storage, pool txpool.TxPool) (*Chain, error) {
	chain := &Chain{
		s:      s,
		txPool: pool,
		cfg:    cfg,
	}
	genesisBlock, err := chain.newGenesisBlock()
	if err != nil {
		return nil, fmt.Errorf("create genesis block err: %w", err)
	}
	chain.curHeader = genesisBlock.Header

	return chain, nil
}

func (c *Chain) GenerateBlock(ctx context.Context, miner account.Address) (*block.Block, error) {
	txs, err := c.txPool.PackTxs()
	if err != nil {
		return nil, fmt.Errorf("pack txs err: %w", err)
	}
	bf, err := bloom.NewFilter(bloomFilterLen)
	if err != nil {
		return nil, fmt.Errorf("new bloom filter err: %w", err)
	}
	for _, tx := range txs {
		txHash, err := hash.CalcHash(&tx)
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

func (c *Chain) AddBlock(ctx context.Context, b *block.Block) error {
	var err error
	if _, err = c.updateTrie(ctx, b.Body.TxList); err != nil {
		return fmt.Errorf("updateTrie err: %w", err)
	}

	var blockHash, blockByte, headerByte []byte
	if blockHash, err = hash.CalcHash(b); err != nil {
		return fmt.Errorf("calc hash err: %w", err)
	}
	if blockByte, err = b.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}
	if blockHash, err = b.Header.Encode(); err != nil {
		return fmt.Errorf("encode block err: %w", err)
	}
	err = c.s.BlockStorage.AddBlock(ctx, blockHash, blockByte, headerByte)
	if err != nil {
		return fmt.Errorf("failed to add block to storage: %w", err)
	}
	return nil
}

func (c *Chain) newGenesisBlock() (*block.Block, error) {
	genesisMiner := account.Address{}
	var b *block.Block
	var err error
	ctx := context.Background()
	if b, err = c.GenerateBlock(ctx, genesisMiner); err != nil {
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
	return c.s.TrieStorage.MAddAccountStatesPreview(ctx, keys, values)
}

func (c *Chain) updateTrie(ctx context.Context, txs []transaction.Transaction) ([]byte, error) {
	keys, values, err := c.getUpdatedAccountsBytes(ctx, txs)
	if err != nil {
		return nil, fmt.Errorf("get updated accounts bytes err: %w", err)
	}
	return c.s.TrieStorage.MAddAccountStatesAndCommit(ctx, keys, values)
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
	keyBytes := make([][]byte, len(txs))
	valBytes := make([][]byte, len(txs))
	var err error
	for i, tx := range txs {
		keyBytes[i], err = hash.CalcHash(&tx)
		if err != nil {
			return nil, fmt.Errorf("hash err: %w", err)
		}
		valBytes[i], err = tx.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode tx err: %w", err)
		}
	}

	return c.s.TrieStorage.GenerateRootByGivenBytes(ctx, keyBytes, valBytes)
}
