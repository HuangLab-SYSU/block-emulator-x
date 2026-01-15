package trie

import "context"

// Store is an MPT-based, append-only structure whose leaf nodes should be considered as accounts.
// The upstream layer of storage not only stores nodes, but provides proofs for the nodes.
type Store interface {
	// GenerateRootByGivenBytes gets the trie root with given keys and values, from an empty trie.
	GenerateRootByGivenBytes(ctx context.Context, keys, values [][]byte) ([]byte, error)
	// GetCurrentRoot returns the root of the trie.
	GetCurrentRoot(ctx context.Context) ([]byte, error)
	// MGetValsByKeys returns the corresponding values with the given keys.
	MGetValsByKeys(ctx context.Context, keys [][]byte) ([][]byte, error)
	// MAddKeyValuesAndCommit adds the given key-value pairs into the trie and commits them into the database.
	MAddKeyValuesAndCommit(ctx context.Context, keys, values [][]byte) ([]byte, error)
	// MAddKeyValuesPreview adds the given key-value pairs into the trie but does not commit them.
	MAddKeyValuesPreview(ctx context.Context, keys, values [][]byte) ([]byte, error)
	// SetStateRoot sets the root of the trie.
	SetStateRoot(ctx context.Context, root []byte) error
	// Close closes the database
	Close() error
}
