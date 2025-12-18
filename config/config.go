package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
)

type Config struct {
	GlobalSys        SystemCfg `json:"system" yaml:"system"`
	ConsensusNodeCfg `json:"consensus_node" yaml:"consensus_node"`
	SupervisorCfg    `json:"supervisor"  yaml:"supervisor"`
	NetworkCfg       `json:"network"    yaml:"network"`
}

type LocalParams struct {
	NodeID     int64
	ShardID    int64
	WalletAddr account.Address
}

// local parameters are defined here, and they are read from command lines.
var (
	//
	localNodeID  = flag.Int64("node_id", -1, "local node id")
	localShardID = flag.Int64("shard_id", -1, "local shard id, 0x7fffffff denotes the supervisor shard")
	accountAddr  = flag.String("account_addr", "", "miner address")
)

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	cfg := new(Config)

	switch filepath.Ext(path) {
	case ".json":
		if err = json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("json unmarshal %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err = yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("yaml unmarshal %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported cfg type")
	}

	// pass the global system config to the supervisor config
	cfg.SystemCfg = cfg.GlobalSys
	// pass the global system config to the blockchain config
	cfg.BlockchainCfg.SystemCfg = cfg.GlobalSys

	return cfg, nil
}

func LoadLocalParams() (*LocalParams, error) {
	walletAddr, err := utils.Hex2Addr(*accountAddr)
	if err != nil {
		return nil, fmt.Errorf("load wallet address: %w", err)
	}

	return &LocalParams{
		NodeID:     *localNodeID,
		ShardID:    *localShardID,
		WalletAddr: walletAddr,
	}, nil
}
