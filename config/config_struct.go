package config

type TxPoolCfg struct {
	Type  string
	Limit int64
}

type BlockchainCfg struct {
	BloomFilterCfg
	StorageCfg
}

type StorageCfg struct {
	BoltCfg
	EthStorageCfg
}

type BoltCfg struct {
	FilePath string `json:"bolt_file_path" yaml:"bolt_file_path"`
}

type EthStorageCfg struct {
	IsMemoryDB    bool   `json:"is_memory_db" yaml:"is_memory_db"`
	LevelFilePath string `json:"level_file_path" yaml:"level_file_path"`
	LevelCache    int    `json:"level_cache" yaml:"level_cache"`
	LevelHandler  int    `json:"level_handler" yaml:"level_handler"`
}

type BloomFilterCfg struct {
	BitsetLen      int
	FilterHashFunc []string
}
