package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type Config struct {
	BlockchainCfg `json:"blockchain" yaml:"blockchain"`
	ConsensusCfg  `json:"consensus" yaml:"consensus"`
	NetworkCfg    `json:"network" yaml:"network"`
	StorageCfg    `json:"storage" yaml:"storage"`
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
