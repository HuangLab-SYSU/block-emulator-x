package chain

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator/config"
)

const (
	chainStorageTestDir = "chain-test"
	bitsetTestLen       = 1 << 10
)

func TestNewChain(t *testing.T) {
	// create test dir
	err := os.MkdirAll(chainStorageTestDir, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	defer clearChainStorage()

	bc, err := NewChain(getTestConfig())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bc.Close())
	}()
}

func clearChainStorage() {
	_ = os.RemoveAll(chainStorageTestDir)
}

func getTestConfig() config.BlockchainCfg {
	return config.BlockchainCfg{
		BloomFilterCfg: config.BloomFilterCfg{
			BitsetLen: bitsetTestLen,
		},
		StorageCfg: config.StorageCfg{
			BlockStorageType: "bolt",
			TrieStorageType:  "eth_level",
			BoltCfg: config.BoltCfg{
				FilePath: path.Join(chainStorageTestDir, "bolt"),
			},
			EthStorageCfg: config.EthStorageCfg{
				IsMemoryDB: true, // memory db for testing
			},
		},
	}
}
