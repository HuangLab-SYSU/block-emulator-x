package storage

import (
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage/trie"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage/vmstate"
)

// Storage consists of  both block.Store and trie.Store.
type Storage struct {
	BlockStorage block.Store    // BlockStorage is used to record block information.
	LocStorage   trie.Store     // LocStorage is used to record the account locations, which is introduced in a sharded blockchain system.
	StateStorage *vmstate.Store // StateStorage is to record the states of accounts and execute contracts.
}

// NewStorage creates a Storage with the given config and local parameters.
func NewStorage(cfg config.StorageCfg, lp config.LocalParams) (*Storage, error) {
	s := &Storage{}

	var err error

	s.StateStorage, err = vmstate.NewVMStateStore(cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("new vm trie db: %w", err)
	}

	switch cfg.BlockStorageType {
	default:
		s.BlockStorage, err = block.NewBoltStore(cfg.BoltCfg, lp)
		if err != nil {
			return nil, fmt.Errorf("new block bolt store: %w", err)
		}
	}

	switch cfg.TrieStorageType {
	default:
		s.LocStorage, err = trie.NewEthereumDefaultTrieDB(cfg.EthStorageCfg, lp)
		if err != nil {
			return nil, fmt.Errorf("new loc trie storage: %w", err)
		}
	}

	return s, nil
}
