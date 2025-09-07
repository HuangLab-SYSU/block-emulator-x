package trie

import "context"

// Store is a mpt-based, append-only structure whose leaf nodes should be considered as accounts.
// The upstream layer of storage not only stores nodes, but provides proofs for the nodes.
type Store interface {
	// GenerateRootByGivenBytes get the trie root with given keys and values, from an empty trie.
	GenerateRootByGivenBytes(ctx context.Context, keys [][]byte, values [][]byte) ([]byte, error)
	// GetCurrentRoot returns the root of the trie.
	GetCurrentRoot(ctx context.Context) ([]byte, error)
	MGetAccountStates(ctx context.Context, keys [][]byte) ([][]byte, error)
	MAddAccountStatesAndCommit(ctx context.Context, keys [][]byte, values [][]byte) ([]byte, error)
	MAddAccountStatesPreview(ctx context.Context, keys [][]byte, values [][]byte) ([]byte, error)
	Close() error
}
