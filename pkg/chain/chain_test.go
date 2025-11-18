package chain

import (
	"context"
	"math/big"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

const (
	chainStorageTestDir = "chain-test"
	bitsetTestLen       = 1 << 10
)

var (
	testMiner     = generateAccountAddr("fake miner")
	testSender    = account.Account{Addr: generateAccountAddr("fake sender    00000")}
	testRecipient = account.Account{Addr: generateAccountAddr("fake recipient 11111")}
	testTxs       = []transaction.Transaction{
		*transaction.NewTransaction(testSender, testRecipient, big.NewInt(100), 0, time.Now()),
	}
)

func TestChain(t *testing.T) {
	cfg := getTestConfig()
	// create test dir
	err := os.MkdirAll(cfg.BoltCfg.FilePathDir, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	defer clearChainStorage()

	// create block
	bc, err := NewChain(getTestConfig(), config.LocalParams{ShardID: 0})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bc.Close())
	}()

	// create block but not add it to the chain
	ctx := context.Background()
	beforeHeader := *bc.GetCurHeader()

	b, err := bc.GenerateBlock(ctx, testMiner, testTxs)
	require.NoError(t, err)
	generateHeader := b.Header

	headerAfterGenerate := *bc.GetCurHeader()
	require.Equal(t, beforeHeader, headerAfterGenerate)
	encodedBeforeHeader, _ := beforeHeader.Encode()
	encodedHeaderAfterHeader, _ := headerAfterGenerate.Encode()
	require.Equal(t, encodedBeforeHeader, encodedHeaderAfterHeader)

	err = bc.AddBlock(ctx, b)
	require.NoError(t, err)

	headerAfterAdd := *bc.GetCurHeader()
	require.NotEqual(t, generateHeader, headerAfterAdd)
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
		ShardNum: 2,
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
