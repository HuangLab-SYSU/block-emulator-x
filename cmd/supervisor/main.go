package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/cmd/loadnetwork"
	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/logger"
	"github.com/HuangLab-SYSU/block-emulator/supervisor"
)

const supervisorWaitingTime = 8 * time.Second

var configPath = flag.String("config", "config.yaml", "path to config file")

func main() {
	flag.Parse()

	lp, err := config.LoadLocalParams()
	if err != nil {
		log.Fatal(fmt.Errorf("config.LoadLocalParams: %w", err))
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(fmt.Errorf("load config: %w", err))
	}

	// set the default logger here
	if err = logger.InitLogger(lp, cfg.LogCfg); err != nil {
		log.Fatal(fmt.Errorf("init logger: %w", err))
	}

	defer logger.CloseLoggerFile()

	p2p, nodeM, err := loadnetwork.GetNetworkAndNodeInfo(lp)
	if err != nil {
		log.Fatal(fmt.Errorf("getNetworkAndNodeTopo: %w", err))
	}

	spv, err := supervisor.NewSupervisor(p2p, nodeM, cfg.SupervisorCfg)
	if err != nil {
		log.Fatal(fmt.Errorf("supervisor.NewSupervisor error: %w", err))
	}

	// start grpc server
	go func() {
		if err = p2p.StartServer(); err != nil {
			log.Fatal(fmt.Errorf("startServer: %w", err))
		}
	}()

	time.Sleep(supervisorWaitingTime)

	if err = spv.Start(); err != nil {
		log.Fatal(fmt.Errorf("supervisor.Startup error: %w", err))
	}
}
