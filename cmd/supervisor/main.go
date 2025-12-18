package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/cmd/loadnetwork"
	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/logger"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/supervisor"
)

const (
	supervisorWaitingTime = 8 * time.Second
	pprofPortLowerBound   = 5000
)

var (
	configPath = flag.String("config", "config.yaml", "path to config file")
	// pprof is a tool to sample the program and collect the runtime data.
	pprofPort = flag.Int("pprof-port", 0, fmt.Sprintf("port to serve pprof; the port should be larger than %d", pprofPortLowerBound))
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

	p2p, nodeM, err := loadnetwork.GetNetworkAndNodeInfo(lp)
	if err != nil {
		log.Fatal(fmt.Errorf("get network and node topology failed: %w", err))
	}

	networkConn := network.NewConnHandler(p2p)

	spv, err := supervisor.NewSupervisor(networkConn, nodeM, cfg.SupervisorCfg)
	if err != nil {
		log.Fatal(fmt.Errorf("new a supervisor failed: %w", err))
	}

	// start gRPC server
	go func() {
		if err = p2p.ListenStart(); err != nil {
			log.Fatal(fmt.Errorf("startServer: %w", err))
		}
	}()

	time.Sleep(supervisorWaitingTime)

	if err = spv.Start(); err != nil {
		log.Fatal(fmt.Errorf("supervisor.Startup error: %w", err))
	}
}
