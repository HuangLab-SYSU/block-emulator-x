package config

type SupervisorCfg struct {
	ShardNum     int64
	TxSource     string
	TxSourceFile string

	TxInjectionSpeed int64 // transactions per second
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
