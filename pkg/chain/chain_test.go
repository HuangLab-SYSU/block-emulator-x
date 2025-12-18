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
		*transaction.NewTransaction(testSender, testRecipient, big.NewInt(100), 0, time.Now()),
	}
)

func TestChain(t *testing.T) {
	cfg := getTestConfig()
	// create test dir
	err := os.MkdirAll(cfg.BoltCfg.FilePathDir, os.ModePerm)
	require.NoError(t, err)

	t.Cleanup(func() { clearChainStorage() })

	// create block
	bc, err := NewChain(getTestConfig(), config.LocalParams{ShardID: 0})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bc.Close())
	}()

	// create block but not add it to the chain
	ctx := context.Background()
	beforeHeader := bc.GetCurHeader()

	b, err := bc.GenerateBlock(ctx, testMiner, testTxs)
	require.NoError(t, err)
	generateHeader := b.Header

	headerAfterGenerate := bc.GetCurHeader()
	// the blockchain's 'curHeader' will not be modified after generating a block
	require.Equal(t, beforeHeader, headerAfterGenerate)

	encodedBeforeHeader, _ := beforeHeader.Encode()
	encodedHeaderAfterGenerate, _ := headerAfterGenerate.Encode()
	// block header in the blockchain is equal in bytes.
	require.Equal(t, encodedBeforeHeader, encodedHeaderAfterGenerate)

	err = bc.AddBlock(ctx, b)
	require.NoError(t, err)

	headerAfterAdd := bc.GetCurHeader()
	require.Equal(t, generateHeader, headerAfterAdd)
	require.NoError(t, err)

	beforeHeaderHash, _ := beforeHeader.Hash()
	require.Equal(t, beforeHeaderHash, generateHeader.ParentBlockHash)

	migratedB, err := bc.GenerateMigrationBlock(ctx, testMiner, []account.Address{testSender}, []account.State{*account.NewState(testSender, 100)})
	require.NoError(t, err)

	err = bc.AddBlock(ctx, migratedB)
	require.NoError(t, err)

	as, err := bc.GetAccountStates(ctx, []account.Address{testSender})
	require.NoError(t, err)
	require.Len(t, as, 1)
	require.Equal(t, *account.NewState(testSender, 100), *as[0])
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
