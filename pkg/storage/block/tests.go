package block

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunComplianceTests run a batch of compliance tests for block.Store.
func RunComplianceTests(t *testing.T, store Store, clear func() error) {
	ctx := context.Background()

	t.Cleanup(func() {
		if err := clear(); err != nil {
			t.Fatalf("failed to remove test directory: %v", err)
		}
	})

	// Predefined test constants
	const (
		blockHash    = "valid-block-hash"
		newBlockHash = "new-valid-block-hash"
		invalidHash  = ""

		fakeBlockByte          = "fake-block"
		fakeBlockHeaderByte    = "fake-block-header"
		newFakeBlockHeaderByte = "new-fake-block-header"
	)

	t.Run("TestBlockStorageInvalidCheck", func(t *testing.T) {
		var err error

		emptyBlockHash := []byte(invalidHash)
		err = store.AddBlock(ctx, emptyBlockHash, []byte(fakeBlockByte), []byte(fakeBlockHeaderByte))
		require.NotNil(t, err)
		err = store.AddBlockHeader(ctx, emptyBlockHash, []byte(fakeBlockHeaderByte))
		require.NotNil(t, err)
	})

	t.Run("TestBlockStorage", func(t *testing.T) {
		// Mock block with fixed hash
		bHash := []byte(blockHash + "-block-test")
		nHash := []byte(newBlockHash + "-block-test")

		// Store block
		err := store.AddBlock(ctx, bHash, []byte(fakeBlockByte), []byte(fakeBlockHeaderByte))
		require.NoError(t, err)

		// Verify retrieval block
		retrievedBlock, err := store.GetBlockByHash(ctx, bHash)
		require.NoError(t, err)
		assert.Equal(t, fakeBlockByte, string(retrievedBlock))

		// Verify retrieval header
		retrievedBlockHeader, err := store.GetBlockHeaderByHash(ctx, bHash)
		require.NoError(t, err)
		assert.Equal(t, fakeBlockHeaderByte, string(retrievedBlockHeader))

		// Verify retrieval the newest
		newestHash, err := store.GetNewestBlockHash(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(bHash), string(newestHash))

		// Store new a new block
		err = store.AddBlockHeader(ctx, nHash, []byte(newFakeBlockHeaderByte))
		require.NoError(t, err)

		// Verify retrieval the newest
		latest, err := store.GetNewestBlockHash(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(nHash), string(latest))
	})

	t.Run("TestBlockHeaderStorage", func(t *testing.T) {
		bHash := []byte(blockHash + "-block-header-test")
		nHash := []byte(newBlockHash + "-block-header-test")

		// Store a header
		err := store.AddBlockHeader(ctx, bHash, []byte(fakeBlockHeaderByte))
		require.NoError(t, err)

		// Verify retrieval the header
		retrievedBlockHeader, err := store.GetBlockHeaderByHash(ctx, bHash)
		require.NoError(t, err)
		assert.Equal(t, fakeBlockHeaderByte, string(retrievedBlockHeader))

		// Verify retrieval the newest
		newestHash, err := store.GetNewestBlockHash(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(bHash), string(newestHash))

		// Store a new header
		err = store.AddBlockHeader(ctx, nHash, []byte(newFakeBlockHeaderByte))
		require.NoError(t, err)

		// Verify retrieval the newest
		latest, err := store.GetNewestBlockHash(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(nHash), string(latest))
	})

	t.Run("TestNewestBlockHashStorage", func(t *testing.T) {
		bHash := []byte(blockHash + "-newest-block-hash-test")

		// Store newest hash
		err := store.UpdateNewestBlockHash(ctx, bHash)
		require.NoError(t, err)

		// Verify retrieval newest
		newestHash, err := store.GetNewestBlockHash(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(bHash), string(newestHash))
	})

	t.Run("TestConcurrency", func(t *testing.T) {
		const numBlocks = 10

		var wg sync.WaitGroup

		wg.Add(numBlocks)

		for i := 0; i < numBlocks; i++ {
			go func(id int) {
				defer wg.Done()

				bHash := []byte(fmt.Sprintf("%s-%d", blockHash, id))
				encodedBlock := []byte(fmt.Sprintf("block-data-%d", id))
				encodedBHeader := []byte(fmt.Sprintf("header-data-%d", id))
				assert.NoError(t, store.AddBlock(ctx, bHash, encodedBlock, encodedBHeader))
			}(i)
		}

		wg.Wait()

		// Verify all blocks exist
		for i := 0; i < numBlocks; i++ {
			bHash := []byte(fmt.Sprintf("%s-%d", blockHash, i))
			data, _ := store.GetBlockByHash(ctx, bHash)
			assert.NotNil(t, data)

			headerData, _ := store.GetBlockHeaderByHash(ctx, bHash)
			assert.NotNil(t, headerData)
		}
	})

	require.NoError(t, store.Close())
}
