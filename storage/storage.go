package storage

import (
	"github.com/HuangLab-SYSU/block-emulator/storage/block"
	"github.com/HuangLab-SYSU/block-emulator/storage/trie"
)

// Storage consists of  both block.Store and trie.Store.
type Storage interface {
	block.Store
	trie.Store
}
