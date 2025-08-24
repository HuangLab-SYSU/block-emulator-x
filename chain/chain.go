package chain

import (
	"context"
	"fmt"
	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/core/block"
	"github.com/HuangLab-SYSU/block-emulator/core/hash"
	"github.com/HuangLab-SYSU/block-emulator/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/storage"
)

type Chain struct {
	curHeader *block.Header
	s         storage.Storage
	txPool    txpool.TxPool
	cfg       config.BlockchainCfg
}

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

func (c *Chain) GenerateBlock() *block.Block {
}

func (c *Chain) AddBlock(ctx context.Context, b *block.Block) error {
	var blockHash, blockByte, headerByte []byte
	var err error
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
	b := c.GenerateBlock()
	if err := c.AddBlock(context.Background(), b); err != nil {
		return nil, fmt.Errorf("failed to add block to storage: %w", err)
	}
	return b, nil
}

func (c *Chain) updateTrie(ctx context.Context, txs []*transaction.Transaction) error {

}
