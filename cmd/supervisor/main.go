package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/HuangLab-SYSU/block-emulator/cmd/loadnetwork"
	"github.com/HuangLab-SYSU/block-emulator/config"
	_ "github.com/HuangLab-SYSU/block-emulator/pkg/logger"
	"github.com/HuangLab-SYSU/block-emulator/supervisor"
)

var configPath = flag.String("config", "config.yaml", "path to config file")

func main() {
	flag.Parse()

	_, p2p, nodeM, err := loadnetwork.GetLocalParamsAndNetworkNodes()
	if err != nil {
		log.Fatal(fmt.Errorf("getNetworkAndNodeTopo: %w", err))
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(fmt.Errorf("load config: %w", err))
	}

	spv, err := supervisor.NewSupervisor(p2p, nodeM, cfg.SupervisorCfg)
	if err != nil {
		log.Fatal(fmt.Errorf("supervisor.NewSupervisor error: %w", err))
	}

	if err = spv.Start(); err != nil {
		log.Fatal(fmt.Errorf("supervisor.Startup error: %w", err))
	}
}
