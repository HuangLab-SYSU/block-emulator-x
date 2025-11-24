package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/cmd/loadnetwork"
	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft"
	"github.com/HuangLab-SYSU/block-emulator/pkg/logger"
)

const nodeWaitingTime = 5 * time.Second

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

	consensusNode, err := pbft.NewPBFTNode(p2p, nodeM, cfg.ConsensusNodeCfg, *lp)
	if err != nil {
		log.Fatal(fmt.Errorf("newPBFTNode: %w", err))
	}

	// start grpc server
	go func() {
		if err = p2p.StartServer(); err != nil {
			log.Fatal(fmt.Errorf("startServer: %w", err))
		}
	}()

	time.Sleep(nodeWaitingTime)

	consensusNode.Start()
}
