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
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/connlibp2p"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
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

	var (
		p2p   network.P2PConn
		nodeM nodetopo.NodeMapper
		err   error
	)

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

	switch cfg.CommunicationMode {
	case config.DirectConnMode:
		p2p, nodeM, err = loadnetwork.GetNetworkAndNodeInfo(cfg.GlobalSys, lp)
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

	case config.LibP2PConnMode:
		p2p, err = loadnetwork.InitNetworkAndNodeInfoWithLibp2pMode(lp)
		if err != nil {
			log.Fatal(fmt.Errorf("get network and node topology failed: %w", err))
		}

		libp2pConn, ok := p2p.(*connlibp2p.Libp2pConn)
		if !ok {
			log.Fatal("unexpected P2PConn type; expected *connlibp2p.Libp2pConn")
		}

		libp2pNodeM := libp2pConn.NodeM
		if libp2pNodeM == nil {
			log.Fatal("the Libp2pConn.NodeM is nil")
		}

		networkConn := network.NewConnHandler(p2p)

		// start gRPC server
		go func() {
			if err = p2p.ListenStart(); err != nil {
				log.Fatal(fmt.Errorf("startServer: %w", err))
			}
		}()

		loadnetwork.WaitForNodeMapperReady(libp2pNodeM, cfg.GlobalSys)

		spv, err := supervisor.NewSupervisor(networkConn, libp2pNodeM, cfg.SupervisorCfg)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create a supervisor node: %w", err))
		}

		time.Sleep(supervisorWaitingTime)

		if err = spv.Start(); err != nil {
			log.Fatal(fmt.Errorf("failed to start supervisor node: %w", err))
		}

	default:
		log.Fatal(fmt.Errorf("unsupported communication mode: %s", cfg.CommunicationMode))
	}
}
