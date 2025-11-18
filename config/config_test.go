package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var expectedCfg = Config{
	BlockchainCfg: BlockchainCfg{
		ShardNum: 4,
		BloomFilterCfg: BloomFilterCfg{
			BitsetLen:      4096,
			FilterHashFunc: []string{"sha256", "sha512", "sha1"},
		},
		StorageCfg: StorageCfg{
			BlockStorageType: "bolt",
			TrieStorageType:  "eth_level_db",
			BoltCfg: BoltCfg{
				FilePathDir: "./exp/boltdb_test/",
			},
			EthStorageCfg: EthStorageCfg{
				IsMemoryDB:       false,
				LevelFilePathDir: "./exp/trie_db_test/",
				OldStateRoot:     "",
			},
		},
	},
	TxPoolCfg: TxPoolCfg{
		Type:  "number",
		Limit: 2000,
	},
	ConsensusCfg: ConsensusCfg{
		ShardNum:      4,
		NodeNum:       4,
		BlockInterval: 5000,
	},
	SupervisorCfg: SupervisorCfg{
		ShardNum:         4,
		TxNumber:         100000,
		TxInjectionSpeed: 10000,
		ResultOutputDir:  "./exp/result_test",
		ConsensusType:    StaticRelayConsensus,
		EpochDuration:    50.0,
		TxSourceCfg: TxSourceCfg{
			TxSourceType: "random_source",
			TxSourceFile: "",
		},
		BrokerModuleCfg: BrokerModuleCfg{
			BrokerFilePath: "./pkg/broker/broker_test",
			BrokerNum:      50,
		},
	},
	NetworkCfg: NetworkCfg{
		Bandwidth: 1000000,
		Latency:   0,
	},
}

func TestLoadConfig(t *testing.T) {
	_, err := LoadConfig("../config")
	require.Error(t, err)
	cfg, err := LoadConfig("./config_test.yaml")
	require.NoError(t, err)
	require.Equal(t, expectedCfg, *cfg)
}
