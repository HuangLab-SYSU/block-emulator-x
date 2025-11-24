package config

const (
	StaticRelayConsensus  = "static_relay"
	StaticBrokerConsensus = "static_broker"
	CLPARelayConsensus    = "clpa_relay"
	CLPABrokerConsensus   = "clpa_broker"
)

type SystemCfg struct {
	ShardNum      int64  `json:"shard_num" yaml:"shard_num"`
	NodeNum       int64  `json:"node_num" yaml:"node_num"`
	ConsensusType string `json:"consensus_type" yaml:"consensus_type"`
	LogCfg        `json:"log" yaml:"log"`
}

type SupervisorCfg struct {
	SystemCfg
	TxNumber         int64  `json:"tx_number" yaml:"tx_number"`
	TxInjectionSpeed int64  `json:"tx_injection_speed" yaml:"tx_injection_speed"` // transactions per second
	ResultOutputDir  string `json:"result_output_dir" yaml:"result_output_dir"`
	EpochDuration    int64  `json:"epoch_duration" yaml:"epoch_duration"`
	TxSourceCfg      `json:"tx_source" yaml:"tx_source"`
	BrokerModuleCfg  `json:"broker_module" yaml:"broker_module"`
}

type ConsensusNodeCfg struct {
	BlockchainCfg `json:"blockchain" yaml:"blockchain"`
	TxPoolCfg     `json:"tx_pool"     yaml:"tx_pool"`
	BlockInterval int64 `json:"block_interval" yaml:"block_interval"` // ms
}

type TxSourceCfg struct {
	TxSourceType string `json:"tx_source_type" yaml:"tx_source_type"`
	TxSourceFile string `json:"tx_source_file" yaml:"tx_source_file"`
}

type BrokerModuleCfg struct {
	BrokerFilePath string `json:"broker_file_path" yaml:"broker_file_path"`
	BrokerNum      int64  `json:"broker_num" yaml:"broker_num"`
}
