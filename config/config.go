package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BlockchainCfg `json:"blockchain" yaml:"blockchain"`
	TxPoolCfg     `json:"txpool"     yaml:"txpool"`
	ConsensusCfg  `json:"consensus"  yaml:"consensus"`
	NetworkCfg    `json:"network"    yaml:"network"`
}

func LoadConfig(path string, config *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	switch filepath.Ext(path) {
	case ".json":
		return json.Unmarshal(data, config)
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, config)
	default:
		return fmt.Errorf("unsupported config type")
	}
}
