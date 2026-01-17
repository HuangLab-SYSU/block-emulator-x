package utils

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
)

func GenerateRootByGivenBytes(keys, values [][]byte) ([]byte, error) {
	// Validate parameters.
	if len(keys) != len(values) {
		return nil, fmt.Errorf(
			"bad input, len(keys) != len(values): len(keys)=%d, len(values)=%d",
			len(keys),
			len(values),
		)
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
