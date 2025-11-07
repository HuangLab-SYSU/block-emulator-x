package config

const (
	StaticRelayConsensus  = "static_relay"
	StaticBrokerConsensus = "static_broker"
	CLPARelayConsensus    = "clpa_relay"
	CLPABrokerConsensus   = "clpa_broker"
)

type SupervisorCfg struct {
	ShardNum         int64
	TxNumber         int64
	TxInjectionSpeed int64 // transactions per second
	ResultOutputDir  string

	TxSourceCfg

	ConsensusType string
	EpochDuration int64 // the time duration for an epoch (second)
}

type TxSourceCfg struct {
	TxSource     string
	TxSourceFile string
}

type ConsensusCfg struct {
	ShardNum int64
	NodeNum  int64

	HandlerBufferSize int64
	BlockInterval     int64 // ms

	LocalSetting
}

type LocalSetting struct {
	NodeID     int64
	ShardID    int64
	WalletAddr [20]byte
	Host       string
}
