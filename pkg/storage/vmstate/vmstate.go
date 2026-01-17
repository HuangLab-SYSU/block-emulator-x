package vmstate

import (
	"fmt"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	vmStateDBFilePathFmt = "vm_state_shard_%d_node_%d"
	vmStateNameSpace     = "vm_state"

	snapshotCacheMB = 1 << 5
	levelDBCacheMB  = 1 << 5
)

func NewStateStore(cfg config.StorageCfg, lp config.LocalParams) (state.Database, error) {
	var trDB *triedb.Database

	if cfg.IsMemoryDB {
		db := rawdb.NewMemoryDatabase()
		trDB = triedb.NewDatabase(rawdb.NewDatabase(db), nil)
	} else {
		level, err := leveldb.New(
			filepath.Join(cfg.LevelFilePathDir, fmt.Sprintf(vmStateDBFilePathFmt, lp.ShardID, lp.NodeID)),
			levelDBCacheMB, 0, vmStateNameSpace, false)
		if err != nil {
			return nil, fmt.Errorf("new StateStorage failed: %w", err)
		}

		trDB = triedb.NewDatabase(rawdb.NewDatabase(level), triedb.HashDefaults)
	}

	sp, err := snapshot.New(snapshot.Config{CacheSize: snapshotCacheMB}, trDB.Disk(), trDB, types.EmptyRootHash)
	if err != nil {
		return nil, fmt.Errorf("failed to new a snapshot: %w", err)
	}

	return state.NewDatabase(trDB, sp), nil
}
