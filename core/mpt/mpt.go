package mpt

import "github.com/HuangLab-SYSU/block-emulator/storage/trie"

type NodeKey [32]byte

// MPT is a tire-based structure to provide proofs of nodes.
type MPT struct {
	TrieStorage trie.Store
}
