package storage

import (
	"fmt"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	vmStateDBFilePathFmt = "vm_state_shard_%d_node_%d"
	vmStateNameSpace     = "vm_state"
)

func newVMTrieDB(cfg config.StorageCfg, lp config.LocalParams) (*triedb.Database, error) {
	level, err := leveldb.New(
		filepath.Join(cfg.LevelFilePathDir, fmt.Sprintf(vmStateDBFilePathFmt, lp.ShardID, lp.NodeID)),
		0, 0, vmStateNameSpace, false)
	if err != nil {
		return nil, fmt.Errorf("new VMTrieDB failed: %w", err)
	}

	return triedb.NewDatabase(rawdb.NewDatabase(level), &triedb.Config{Preimages: true, IsVerkle: false}), nil
}
