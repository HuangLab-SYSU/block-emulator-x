package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var expectedSystemCfg = SystemCfg{
	ShardNum:       4,
	NodeNum:        4,
	ConsensusType:  StaticRelayConsensus,
	BlockSizeLimit: 2000,
	LogCfg: LogCfg{
		LogDir:   "./exp_test/",
		LogLevel: "debug",
	},
}

var expectedCfg = Config{
	GlobalSys: expectedSystemCfg,
	ConsensusNodeCfg: ConsensusNodeCfg{
		BlockchainCfg: BlockchainCfg{
			SystemCfg: expectedSystemCfg,
			BloomFilterCfg: BloomFilterCfg{
				BitsetLen:      4096,
				FilterHashFunc: []string{"sha256", "sha512", "sha1"},
			},
			StorageCfg: StorageCfg{
				BlockStorageType: "bolt",
				TrieStorageType:  "eth_level_db",
				BoltCfg: BoltCfg{
					FilePathDir: "./exp_test/boltdb/",
				},
				EthStorageCfg: EthStorageCfg{
					IsMemoryDB:       false,
					LevelFilePathDir: "./exp_test/trie_db/",
					OldStateRoot:     "",
				},
			},
		},
		TxPoolCfg: TxPoolCfg{
			Type: "number",
		},
		BlockInterval:  5000,
		BlockRecordDir: "./exp_test/block_record/",
	},
	SupervisorCfg: SupervisorCfg{
		TxNumber:         100000,
		TxInjectionSpeed: 10000,
		ResultOutputDir:  "./exp_test/results/",
		EpochDuration:    50.0,
		TxSourceCfg: TxSourceCfg{
			TxSourceType: "random_source",
			TxSourceFile: "",
		},
		BrokerModuleCfg: BrokerModuleCfg{
			BrokerFilePath: "./pkg/broker/broker",
			BrokerNum:      50,
		},
		SystemCfg: expectedSystemCfg,
	},
	NetworkCfg: NetworkCfg{
		Bandwidth:         1000000,
		Latency:           0,
		CommunicationMode: "libp2p",
		LibP2PConnCfg: LibP2PConnCfg{
			BootstrapKeyFp: "./pkg/network/connlibp2p/bootstrap.key",
			BootstrapPeer:  "12D3KooWR6siPMZ2sMFKbgwaJFwQfnKczuPZnxHfyy1dHTzZSAUY",
			BootstrapIP:    "127.0.0.1",
			BootstrapPort:  12345,
		},
	},
}

func TestLoadConfig(t *testing.T) {
	_, err := LoadConfig("../config")
	require.Error(t, err)
	cfg, err := LoadConfig("./config_test.yaml")
	require.NoError(t, err)
	require.Equal(t, expectedCfg, *cfg)
}
