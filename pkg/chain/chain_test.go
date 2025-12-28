package chain

import (
	"context"
	"math/big"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
)

const (
	chainStorageTestDir = "chain-test"
	bitsetTestLen       = 1 << 10
)

var (
	testMiner     = generateAccountAddr("fake miner")
	testSender    = generateAccountAddr("fake sender    00000")
	testRecipient = generateAccountAddr("fake recipient 11111")
	testTxs       = []transaction.Transaction{
		*transaction.NewTransaction(testSender, testRecipient, big.NewInt(100), big.NewInt(0), 0, time.Now()),
	}
)

func TestChain(t *testing.T) {
	cfg := getTestConfig()
	// Create the test dir.
	err := os.MkdirAll(cfg.BoltCfg.FilePathDir, os.ModePerm)
	require.NoError(t, err)

	t.Cleanup(func() { clearChainStorage() })

	// Create a blockchain.
	bc, err := NewChain(getTestConfig(), config.LocalParams{ShardID: 0})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bc.Close())
	}()

	b1 := generateBlockButNotAdd(t, bc)

	// Add this block to the chain.
	validateAndAddBlock(t, bc, b1)

	// The test to generate and add a migration block
	addMigrationBlockAndCheck(t, bc)

	// The test to get blocks by the given beginHeight
	fetchBlocksCheck(t, bc)
}

func clearChainStorage() {
	_ = os.RemoveAll(chainStorageTestDir)
}

func generateAccountAddr(s string) account.Address {
	var addr account.Address
	copy(addr[:], s)
	return addr
}

func getTestConfig() config.BlockchainCfg {
	return config.BlockchainCfg{
		SystemCfg: config.SystemCfg{
			ShardNum: 4,
		},
		BloomFilterCfg: config.BloomFilterCfg{
			BitsetLen: bitsetTestLen,
		},
		StorageCfg: config.StorageCfg{
			BlockStorageType: "bolt",
			TrieStorageType:  "eth_level",
			BoltCfg: config.BoltCfg{
				FilePathDir: path.Join(chainStorageTestDir, "bolt"),
			},
			EthStorageCfg: config.EthStorageCfg{
				IsMemoryDB: true, // memory db for testing
			},
		},
	}
}

func generateBlockButNotAdd(t *testing.T, bc *Chain) *block.Block {
	ctx := context.Background()

	headerBeforeGeneration := bc.GetCurHeader()
	expectedParentHash, err := headerBeforeGeneration.Hash()
	require.NoError(t, err)

	// Generate a block but not add it.
	b, err := bc.GenerateBlock(ctx, testMiner, block.TxBlockType, block.Body{TxList: testTxs}, block.MigrationOpt{})
	require.NoError(t, err)

	headerAfterGeneration := bc.GetCurHeader()
	// The blockchain's 'curHeader' will not be modified after generating a block.
	require.Equal(t, headerBeforeGeneration, headerAfterGeneration)

	encodedHeaderBeforeGen, _ := headerBeforeGeneration.Encode()
	encodedHeaderAfterGen, _ := headerAfterGeneration.Encode()
	// Block header in the blockchain is equal in bytes.
	require.Equal(t, encodedHeaderBeforeGen, encodedHeaderAfterGen)

	// The block's parent hash should be equal to expectedParentHash.
	require.Equal(t, expectedParentHash, b.ParentBlockHash)
	return b
}

func validateAndAddBlock(t *testing.T, bc *Chain, b *block.Block) {
	ctx := context.Background()
	err := bc.ValidateBlock(ctx, b)
	require.NoError(t, err)
	err = bc.AddBlock(ctx, b)
	require.NoError(t, err)
}

func addMigrationBlockAndCheck(t *testing.T, bc *Chain) {
	ctx := context.Background()
	const testLoc = 100

	oldHeaderHash, err := bc.GetCurHeader().Hash()
	require.NoError(t, err)

	// Generate a migration block.
	migratedB, err := bc.GenerateBlock(ctx, testMiner, block.TxBlockType, block.Body{},
		block.MigrationOpt{
			MigratedAccounts: []account.Address{testSender},
			MigratedStates:   []account.State{*account.NewState(testSender, testLoc)},
		},
	)
	require.NoError(t, err)

	// Check the parent hash of this migration block.
	require.Equal(t, oldHeaderHash, migratedB.ParentBlockHash)

	// Add this block to the chain.
	err = bc.AddBlock(ctx, migratedB)
	require.NoError(t, err)

	// Get the states of the test account: the state should be equal to the expect one.
	as, err := bc.GetAccountStates(ctx, []account.Address{testSender})
	require.NoError(t, err)
	require.Len(t, as, 1)
	require.Equal(t, *account.NewState(testSender, testLoc), *as[0])
}

func fetchBlocksCheck(t *testing.T, bc *Chain) {
	ctx := context.Background()
	require.Equal(t, uint64(3), bc.GetCurHeader().Number)
	blocks, err := bc.GetBlocksAfterHeight(ctx, -1)
	require.Error(t, err)

	blocks, err = bc.GetBlocksAfterHeight(ctx, 2)
	require.NoError(t, err)
	require.Len(t, blocks, 2)
}
