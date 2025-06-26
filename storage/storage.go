package storage

import (
	"github.com/HuangLab-SYSU/block-emulator/storage/block"
	"github.com/HuangLab-SYSU/block-emulator/storage/trie"
)

// Storage consists of  both block.Store and trie.Store.
type Storage struct {
	BlockStorage block.Store
	TrieStorage  trie.Store
}

func NewStorage(b block.Store, t trie.Store) *Storage {
	return &Storage{
		BlockStorage: b,
		TrieStorage:  t,
	}
}
