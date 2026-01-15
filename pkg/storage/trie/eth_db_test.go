package trie

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

var emptyLocalParams = config.LocalParams{}

func TestEthereumDefaultTrieBasicFlow(t *testing.T) {
	ctx := context.Background()

	// Create the memory trie DB
	tdb, err := NewEthereumDefaultTrieDB(config.EthStorageCfg{IsMemoryDB: true}, emptyLocalParams)
	require.NoError(t, err)

	// The initial root should be empty
	root, err := tdb.GetCurrentRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, types.EmptyRootHash.Bytes(), root)

	// Preview the root
	keys := [][]byte{[]byte("alice"), []byte("bob")}
	vals := [][]byte{[]byte("100"), []byte("200")}

	previewRoot, err := tdb.MAddKeyValuesPreview(ctx, keys, vals)
	require.NoError(t, err)
	require.NotEqual(t, types.EmptyRootHash.Bytes(), previewRoot)

	// The preview operation will not add the nodes
	values, err := tdb.MGetValsByKeys(ctx, keys)
	require.NoError(t, err)
	require.Empty(t, values[0])
	require.Empty(t, values[1])

	notCommitRoot, err := tdb.GetCurrentRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, types.EmptyRootHash.Bytes(), notCommitRoot)

	// Commit
	committedRoot, err := tdb.MAddKeyValuesAndCommit(ctx, keys, vals)
	require.NoError(t, err)
	require.Equal(t, previewRoot, committedRoot)

	// Get the updated states
	retrieved, err := tdb.MGetValsByKeys(ctx, keys)
	require.NoError(t, err)
	require.Equal(t, vals[0], retrieved[0])
	require.Equal(t, vals[1], retrieved[1])

	// Set to the old trie
	err = tdb.SetStateRoot(ctx, notCommitRoot)
	require.NoError(t, err)

	retrieved, err = tdb.MGetValsByKeys(ctx, keys)
	require.NoError(t, err)
	require.Nil(t, retrieved[0])
	require.Nil(t, retrieved[1])
}

func TestEthereumDefaultTrieEmptyAndMismatchInputs(t *testing.T) {
	// Create the memory trie DB
	tdb, err := NewEthereumDefaultTrieDB(config.EthStorageCfg{IsMemoryDB: true}, emptyLocalParams)
	require.NoError(t, err)
	ctx := context.Background()

	// Empty key/value
	root, err := tdb.GenerateRootByGivenBytes(ctx, [][]byte{}, [][]byte{})
	require.NoError(t, err)
	require.Equal(t, types.EmptyRootHash.Bytes(), root)

	// Bad inputs
	_, err = tdb.GenerateRootByGivenBytes(ctx, [][]byte{[]byte("a")}, [][]byte{})
	require.Error(t, err)
	_, err = tdb.MAddKeyValuesAndCommit(ctx, [][]byte{[]byte("a")}, [][]byte{})
	require.Error(t, err)
	_, err = tdb.MAddKeyValuesPreview(ctx, [][]byte{[]byte("a")}, [][]byte{})
	require.Error(t, err)

	require.NoError(t, tdb.Close())
}

func TestEthereumDefaultTrieGetUnknownKey(t *testing.T) {
	// Create the memory trie DB
	tdb, err := NewEthereumDefaultTrieDB(config.EthStorageCfg{IsMemoryDB: true}, emptyLocalParams)
	require.NoError(t, err)
	ctx := context.Background()
	vals, err := tdb.MGetValsByKeys(ctx, [][]byte{[]byte("unknown")})
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Nil(t, vals[0])
}
