package storage

import (
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/storage/block"
	"github.com/HuangLab-SYSU/block-emulator/storage/trie"
)

// Storage consists of  both block.Store and trie.Store.
type Storage struct {
	BlockStorage block.Store
	TrieStorage  trie.Store
}

func NewStorage(cfg config.StorageCfg) (*Storage, error) {
	s := &Storage{}
	var err error
	switch cfg.BlockStorageType {
	default:
		s.BlockStorage, err = block.NewBoltStore(cfg.BoltCfg)
		if err != nil {
			return nil, fmt.Errorf("NewBoltStore: %v", err)
		}
	}

	switch cfg.TrieStorageType {
	default:
		s.TrieStorage, err = trie.NewEthereumDefaultTrieDB(cfg.EthStorageCfg)
		if err != nil {
			return nil, fmt.Errorf("NewEthereumDefaultTrieDB: %v", err)
		}
	}
	return s, nil
}
