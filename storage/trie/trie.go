package trie

import "context"

// Store is a trie-based, append-only structure whose leaf nodes should be considered as accounts.
// The upstream layer of storage not only stores nodes, but provides proofs for the nodes.
type Store interface {
	GetNodeByHash(ctx context.Context, hash []byte) ([]byte, error)
	AddNode(ctx context.Context, hash []byte) error
}
