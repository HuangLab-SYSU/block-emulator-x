package storage

import (
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage/trie"
)

// Storage consists of  both block.Store and trie.Store.
type Storage struct {
	BlockStorage block.Store
	TrieStorage  trie.Store
}

func NewStorage(cfg config.StorageCfg, lp config.LocalParams) (*Storage, error) {
	s := &Storage{}

	var err error

	switch cfg.BlockStorageType {
	default:
		s.BlockStorage, err = block.NewBoltStore(cfg.BoltCfg, lp)
		if err != nil {
			return nil, fmt.Errorf("NewBoltStore: %w", err)
		}
	}

	switch cfg.TrieStorageType {
	default:
		s.TrieStorage, err = trie.NewEthereumDefaultTrieDB(cfg.EthStorageCfg, lp)
		if err != nil {
			return nil, fmt.Errorf("NewEthereumDefaultTrieDB: %w", err)
		}
	}

	return s, nil
}
