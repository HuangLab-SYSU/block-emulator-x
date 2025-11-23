package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/HuangLab-SYSU/block-emulator/cmd/loadnetwork"
	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft"
)

var configPath = flag.String("config", "config.yaml", "path to config file")

func main() {
	flag.Parse()

	lp, p2p, nodeM, err := loadnetwork.GetLocalParamsAndNetworkNodes()
	if err != nil {
		log.Fatal(fmt.Errorf("getNetworkAndNodeTopo: %w", err))
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(fmt.Errorf("load config: %w", err))
	}

	consensusNode, err := pbft.NewPBFTNode(p2p, nodeM, cfg.ConsensusNodeCfg, *lp)
	if err != nil {
		log.Fatal(fmt.Errorf("newPBFTNode: %w", err))
	}

	consensusNode.Start()
}
