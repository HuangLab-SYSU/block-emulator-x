package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/cmd/loadnetwork"
	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/logger"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"

	_ "net/http/pprof"
)

const (
	nodeWaitingTime     = 5 * time.Second
	pprofPortLowerBound = 5000
)

var (
	configPath = flag.String("config", "config.yaml", "path to config file")
	// pprof is a tool to sample the program and collect the runtime data.
	pprofPort = flag.Int(
		"pprof-port",
		0,
		fmt.Sprintf("port to serve pprof; the port should be larger than %d", pprofPortLowerBound),
	)
)

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

	if pprofPort != nil && *pprofPort >= pprofPortLowerBound {
		go func() { log.Println(http.ListenAndServe(fmt.Sprintf(":%d", *pprofPort), nil)) }()
	}

	// set the default logger here
	if err = logger.InitLogger(lp, cfg.LogCfg); err != nil {
		log.Fatal(fmt.Errorf("init logger failed: %w", err))
	}

	defer logger.CloseLoggerFile()

	p2p, nodeM, err := loadnetwork.PrepareNetworkByCfg(cfg, lp)
	if err != nil {
		log.Fatal(fmt.Errorf("prepare network: %w", err))
	}

	consensusNode, err := pbft.NewPBFTNode(network.NewConnHandler(p2p), nodeM, cfg.ConsensusNodeCfg, *lp)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to create a PBFT node: %w", err))
	}

	// Wait other nodes to start listening.
	time.Sleep(nodeWaitingTime)

	consensusNode.Start()
}
