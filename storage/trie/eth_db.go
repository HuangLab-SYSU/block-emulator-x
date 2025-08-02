package trie

import (
	"context"
	"fmt"
	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
	"log/slog"
)

const levelNamespace = "trie"

type EthereumDefaultTrieDB struct {
	trieDB       *triedb.Database
	curStateRoot common.Hash
}

func NewEthereumDefaultTrieDB(cfg *config.EthStorageCfg, oldStateRoot []byte) (*EthereumDefaultTrieDB, error) {
	var db ethdb.Database
	if cfg.IsMemoryDB {
		db = rawdb.NewMemoryDatabase()
	} else {
		level, err := leveldb.New(
			cfg.LevelFilePath,
			cfg.LevelCache,
			cfg.LevelHandler,
			levelNamespace,
			false,
		)
		if err != nil {
			slog.Error("Failed to open level db", "err", err)
			return nil, err
		}
		db = rawdb.NewDatabase(level)
	}

	trieDb := triedb.NewDatabase(db, &triedb.Config{
		Preimages: true,
		IsVerkle:  false,
	})

	trieId := trie.TrieID(types.EmptyRootHash)
	// if there are existing merkle, try to re-build it.
	if oldStateRoot != nil {
		trieId = trie.TrieID(common.BytesToHash(oldStateRoot))
		// make sure that the old trie can be built.
		_, err := trie.New(trieId, trieDb)
		if err != nil {
			slog.Error("Failed to create old eth trie", "err", err)
			return nil, err
		}
	}

	return &EthereumDefaultTrieDB{trieDB: trieDb, curStateRoot: trieId.StateRoot}, nil
}

func (e *EthereumDefaultTrieDB) GenerateRootByGivenBytes(ctx context.Context, keys [][]byte, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		retErr := fmt.Errorf("GenerateRootByGivenBytes failed, len(keys)=%d, len(values)=%d, len(keys) != len(values)", len(keys), len(values))
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}
	// Create a new trie.
	memTrieDb := triedb.NewDatabase(rawdb.NewMemoryDatabase(), &triedb.Config{IsVerkle: false})
	tempTrie := trie.NewEmpty(memTrieDb)
	for i := range keys {
		err := tempTrie.Update(keys[i], values[i])
		if err != nil {
			retErr := fmt.Errorf("GenerateRootByGivenBytes call trie.Update failed, key=%s, err=%v", string(keys[i]), err)
			slog.ErrorContext(ctx, retErr.Error())
			return nil, retErr
		}
	}
	return tempTrie.Hash().Bytes(), nil
}

func (e *EthereumDefaultTrieDB) GetCurrentRoot(ctx context.Context) ([]byte, error) {
	return e.curStateRoot.Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MAddAccountStatesPreview(ctx context.Context, keys [][]byte, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		retErr := fmt.Errorf("MAddAccountStatesPreview failed, len(keys)=%d, len(values)=%d, len(keys) != len(values)", len(keys), len(values))
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}
	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		retErr := fmt.Errorf("MAddAccountStatesPreview call trie.New failed, err=%v", err)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}

	for i := range keys {
		err := curTrie.Update(keys[i], values[i])
		if err != nil {
			retErr := fmt.Errorf("MAddAccountStatesPreview call trie.Update failed, key=%s, err=%v", string(keys[i]), err)
			slog.ErrorContext(ctx, retErr.Error())
			return nil, retErr
		}
	}
	return curTrie.Hash().Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MAddAccountStatesAndCommit(ctx context.Context, keys [][]byte, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		retErr := fmt.Errorf("MAddAccountStatesAndCommit failed, len(keys)=%d, len(values)=%d, len(keys) != len(values)", len(keys), len(values))
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}

	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		retErr := fmt.Errorf("MAddAccountStatesAndCommit call trie.New failed, err=%v", err)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}

	for i := range keys {
		err := curTrie.Update(keys[i], values[i])
		if err != nil {
			retErr := fmt.Errorf("MAddAccountStatesAndCommit call trie.Update failed, key=%s, err=%v", string(keys[i]), err)
			slog.ErrorContext(ctx, retErr.Error())
			return nil, retErr
		}
	}

	newRoot, nodeSet := curTrie.Commit(true)
	err = e.trieDB.Update(newRoot, e.curStateRoot, 0, trienode.NewWithNodeSet(nodeSet), nil)
	if err != nil {
		retErr := fmt.Errorf("MAddAccountStatesAndCommit call triedb.Update failed, err=%v", err)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}
	e.curStateRoot = newRoot

	return e.curStateRoot.Bytes(), nil
}

func (e *EthereumDefaultTrieDB) MGetAccountStates(ctx context.Context, keys [][]byte) ([][]byte, error) {
	curTrie, err := trie.New(trie.TrieID(e.curStateRoot), e.trieDB)
	if err != nil {
		retErr := fmt.Errorf("MGetAccountStates call trie.New failed, err=%v", err)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}

	ret := make([][]byte, len(keys))
	for i, key := range keys {
		val, err := curTrie.Get(key)
		ret[i] = val
		if err != nil {
			retErr := fmt.Errorf("MGetAccountStates call trie.Get failed, err=%v", err)
			slog.ErrorContext(ctx, retErr.Error())
			return nil, retErr
		}
	}
	return ret, nil
}
