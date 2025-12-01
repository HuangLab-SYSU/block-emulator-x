package config

const (
	TxPoolNumType  = "number"
	TxPoolByteType = "byte"
)

type LogCfg struct {
	LogDir   string `json:"log_dir" yaml:"log_dir"`
	LogLevel string `json:"log_level" yaml:"log_level"`
}

type TxPoolCfg struct {
	Type string `json:"type" yaml:"type"`
}

type BlockchainCfg struct {
	SystemCfg
	BloomFilterCfg `json:"bloom_filter" yaml:"bloom_filter"`
	StorageCfg     `json:"storage" yaml:"storage"`
}

type StorageCfg struct {
	BlockStorageType string `json:"block_storage_type" yaml:"block_storage_type"`
	TrieStorageType  string `json:"trie_storage_type" yaml:"trie_storage_type"`
	BoltCfg          `json:"bolt" yaml:"bolt"`
	EthStorageCfg    `json:"eth_storage" yaml:"eth_storage"`
}

type BoltCfg struct {
	FilePathDir string `json:"file_path_dir" yaml:"file_path_dir"`
}

type EthStorageCfg struct {
	IsMemoryDB       bool   `json:"is_memory_db"    yaml:"is_memory_db"`
	LevelFilePathDir string `json:"level_file_path_dir" yaml:"level_file_path_dir"`
	OldStateRoot     string `json:"old_state_root"   yaml:"old_state_root"`
}

type BloomFilterCfg struct {
	BitsetLen      int      `json:"bitset_len" yaml:"bitset_len"`
	FilterHashFunc []string `json:"filter_hash_func" yaml:"filter_hash_func"`
}
