package trie

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator/config"
)

const levelDBNamespace = "trie"

type EthereumDefaultTrieDB struct {
	trieDB       *triedb.Database
	curStateRoot common.Hash
}

func NewEthereumDefaultTrieDB(cfg config.EthStorageCfg) (*EthereumDefaultTrieDB, error) {
	var db ethdb.Database
	if cfg.IsMemoryDB {
		db = rawdb.NewMemoryDatabase()
	} else {
		level, err := leveldb.New(
			cfg.LevelFilePath,
			cfg.LevelCache,
			cfg.LevelHandler,
			levelDBNamespace,
			false,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open level db: %w", err)
		}
		db = rawdb.NewDatabase(level)
	}

	trieDb := triedb.NewDatabase(db, &triedb.Config{
		Preimages: true,
		IsVerkle:  false,
	})

	trieId := trie.TrieID(types.EmptyRootHash)
	// if there are existing merkle, try to re-build it.
	if cfg.OldStateRoot != nil {
		trieId = trie.TrieID(common.BytesToHash(cfg.OldStateRoot))
		// make sure that the old trie can be built.
		_, err := trie.New(trieId, trieDb)
		if err != nil {
			return nil, fmt.Errorf("failed to create old eth trie: %w", err)
		}
	}
	return &EthereumDefaultTrieDB{trieDB: trieDb, curStateRoot: trieId.StateRoot}, nil
}

func (_ *EthereumDefaultTrieDB) GenerateRootByGivenBytes(_ context.Context, keys [][]byte, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		return nil, fmt.Errorf("bad input, len(keys) != len(values): len(keys)=%d, len(values)=%d", len(keys), len(values))
	}
	// Create a new trie.
	memTrieDb := triedb.NewDatabase(rawdb.NewMemoryDatabase(), &triedb.Config{IsVerkle: false})
	tempTrie := trie.NewEmpty(memTrieDb)
	for i := range keys {
		err := tempTrie.Update(keys[i], values[i])
		if err != nil {
			return nil, fmt.Errorf("update trie db failed, err=%w", err)
		}
	}
	return tempTrie.Hash().Bytes(), nil
}

func (e *EthereumDefaultTrieDB) GetCurrentRoot(_ context.Context) ([]byte, error) {
	return e.curStateRoot.Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MAddAccountStatesPreview(_ context.Context, keys [][]byte, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		return nil, fmt.Errorf("bad input, len(keys) != len(values): len(keys)=%d, len(values)=%d", len(keys), len(values))
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

func (e *EthereumDefaultTrieDB) MAddAccountStatesAndCommit(_ context.Context, keys [][]byte, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		return nil, fmt.Errorf("bad input, len(keys) != len(values): len(keys)=%d, len(values)=%d", len(keys), len(values))
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

	newRoot, nodeSet := curTrie.Commit(true)
	if nodeSet == nil {
		// no dirty nodes
		return e.curStateRoot.Bytes(), nil
	}
	err = e.trieDB.Update(newRoot, e.curStateRoot, 0, trienode.NewWithNodeSet(nodeSet), nil)
	if err != nil {
		return nil, fmt.Errorf("update trie db failed, err=%w", err)
	}
	e.curStateRoot = newRoot

	return e.curStateRoot.Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MGetAccountStates(_ context.Context, keys [][]byte) ([][]byte, error) {
	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		return nil, fmt.Errorf("new trie failed, err=%w", err)
	}

	ret := make([][]byte, len(keys))
	for i, key := range keys {
		val, err := curTrie.Get(key)
		if err != nil {
			return nil, fmt.Errorf("get trie failed, err=%w", err)
		}
		ret[i] = val
	}
	return ret, nil
}

func (e *EthereumDefaultTrieDB) Close() error {
	return e.trieDB.Close()
}
