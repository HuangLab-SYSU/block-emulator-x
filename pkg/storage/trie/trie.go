package trie

import "context"

// Store is an MPT-based, append-only structure whose leaf nodes should be considered as accounts.
// The upstream layer of storage not only stores nodes, but provides proofs for the nodes.
type Store interface {
	// GenerateRootByGivenBytes gets the trie root with given keys and values, from an empty trie.
	GenerateRootByGivenBytes(ctx context.Context, keys, values [][]byte) ([]byte, error)
	// GetCurrentRoot returns the root of the trie.
	GetCurrentRoot(ctx context.Context) ([]byte, error)
	// MGetAccountStates returns the corresponding values with the given keys.
	MGetAccountStates(ctx context.Context, keys [][]byte) ([][]byte, error)
	// MAddAccountStatesAndCommit adds the given key-value pairs into the trie and commits them into the database.
	MAddAccountStatesAndCommit(ctx context.Context, keys, values [][]byte) ([]byte, error)
	// MAddAccountStatesPreview adds the given key-value pairs into the trie but does not commit them.
	MAddAccountStatesPreview(ctx context.Context, keys, values [][]byte) ([]byte, error)
	// SetStateRoot sets the root of the trie.
	SetStateRoot(ctx context.Context, root []byte) error
	// Close closes the database
	Close() error
}
