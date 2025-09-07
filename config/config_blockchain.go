package config

type BlockchainCfg struct {
	TxPoolCfg
}

type TxPoolCfg struct {
	Type  string
	Limit int64
}
