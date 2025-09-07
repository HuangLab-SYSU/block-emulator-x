package config

type BlockchainCfg struct {
	BloomFilterCfg
}

type TxPoolCfg struct {
	Type  string
	Limit int64
}

type BloomFilterCfg struct {
	BitsetLen      int
	FilterHashFunc []string
}
