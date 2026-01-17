package trie

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	levelDBNamespace   = "shard_loc_trie"
	defaultLevelCache  = 1 << 5
	levelDBFilePathFmt = "account_loc_shard_%d_node_%d"
)

type EthereumDefaultTrieDB struct {
	trieDB       *triedb.Database
	curStateRoot common.Hash
}

func NewEthereumDefaultTrieDB(cfg config.EthStorageCfg, lp config.LocalParams) (*EthereumDefaultTrieDB, error) {
	var db ethdb.Database
	if cfg.IsMemoryDB {
		db = rawdb.NewMemoryDatabase()
	} else {
		level, err := leveldb.New(
			filepath.Join(cfg.LevelFilePathDir, fmt.Sprintf(levelDBFilePathFmt, lp.ShardID, lp.NodeID)),
			defaultLevelCache, 0, levelDBNamespace, false)
		if err != nil {
			return nil, fmt.Errorf("failed to open level db: %w", err)
		}

		db = rawdb.NewDatabase(level)
	}

	trieDb := triedb.NewDatabase(db, triedb.HashDefaults)

	trieId := trie.TrieID(types.EmptyRootHash)
	// if there are existing merkle, try to re-build it.
	if cfg.OldStateRoot != "" {
		trieId = trie.TrieID(common.BytesToHash([]byte(cfg.OldStateRoot)))
		// make sure that the old trie can be built.
		_, err := trie.New(trieId, trieDb)
		if err != nil {
			return nil, fmt.Errorf("failed to create old eth trie: %w", err)
		}
	}

	return &EthereumDefaultTrieDB{trieDB: trieDb, curStateRoot: trieId.StateRoot}, nil
}

func (e *EthereumDefaultTrieDB) GetCurrentRoot(_ context.Context) ([]byte, error) {
	return e.curStateRoot.Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MAddKeyValuesPreview(_ context.Context, keys, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		return nil, fmt.Errorf(
			"bad input, len(keys) != len(values): len(keys)=%d, len(values)=%d",
			len(keys),
			len(values),
		)
	}

	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		return nil, fmt.Errorf("new trie failed, err=%w", err)
	}

	for i := range keys {
		err := curTrie.Update(keys[i], values[i])
		if err != nil {
			return nil, fmt.Errorf("update trie db failed, err=%w", err)
		}
	}

	return curTrie.Hash().Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MAddKeyValuesAndCommit(_ context.Context, keys, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		return nil, fmt.Errorf(
			"bad input, len(keys) != len(values): len(keys)=%d, len(values)=%d",
			len(keys),
			len(values),
		)
	}

	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		return nil, fmt.Errorf("new trie failed, err=%w", err)
	}

	for i := range keys {
		err = curTrie.Update(keys[i], values[i])
		if err != nil {
			return nil, fmt.Errorf("update current trie failed, err=%w", err)
		}
	}

	newRoot, nodeSet := curTrie.Commit(false) // must be false here
	if nodeSet == nil {                       // no dirty nodes
		return e.curStateRoot.Bytes(), nil
	}

	err = e.trieDB.Update(newRoot, e.curStateRoot, 0, trienode.NewWithNodeSet(nodeSet), nil)
	if err != nil {
		return nil, fmt.Errorf("update trie db failed, err=%w", err)
	}

	e.curStateRoot = newRoot
	if err = e.trieDB.Commit(newRoot, true); err != nil {
		return nil, fmt.Errorf("commit trie db failed, err=%w", err)
	}

	return e.curStateRoot.Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MGetValsByKeys(_ context.Context, keys [][]byte) ([][]byte, error) {
	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		return nil, fmt.Errorf("new trie failed, err=%w", err)
	}

	ret := make([][]byte, len(keys))

	for i, key := range keys {
		ret[i], err = curTrie.Get(key)
		if err != nil {
			return nil, fmt.Errorf("get trie failed, err=%w", err)
		}
	}

	return ret, nil
}

func (e *EthereumDefaultTrieDB) SetStateRoot(_ context.Context, root []byte) error {
	newRoot := common.BytesToHash(root)

	_, err := trie.New(trie.TrieID(newRoot), e.trieDB)
	if err != nil {
		return fmt.Errorf("new trie failed, err=%w", err)
	}

	e.curStateRoot = newRoot

	return nil
}

func (e *EthereumDefaultTrieDB) Close() error {
	return e.trieDB.Close()
}
