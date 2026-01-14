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
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/logger"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/connlibp2p"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

const (
	nodeWaitingTime     = 5 * time.Second
	pprofPortLowerBound = 5000
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

		consensusNode, err := pbft.NewPBFTNode(networkConn, nodeM, cfg.ConsensusNodeCfg, *lp)
		if err != nil {
			log.Fatal(fmt.Errorf("new a PBFT node failed: %w", err))
		}

		// start gRPC server
		go func() {
			if err = p2p.ListenStart(); err != nil {
				log.Panic(fmt.Errorf("failed to start listening: %w", err))
			}
		}()

		time.Sleep(nodeWaitingTime)

		consensusNode.Start()

	case config.LibP2PConnMode:
		p2p, err = loadnetwork.InitNetworkAndNodeInfoWithLibP2PMode(lp)
		if err != nil {
			log.Fatal(fmt.Errorf("get network and node topology failed: %w", err))
		}

		libP2PConn, ok := p2p.(*connlibp2p.LibP2PConn)
		if !ok {
			log.Fatal("unexpected P2PConn type; expected *connlibp2p.LibP2PConn")
		}

		libP2PNodeM := libP2PConn.NodeM
		if libP2PNodeM == nil {
			log.Fatal("the LibP2PConn.NodeM is nil")
		}

		networkConn := network.NewConnHandler(p2p)
		// start gRPC server
		go func() {
			if err = p2p.ListenStart(); err != nil {
				log.Panic(fmt.Errorf("failed to start listening: %w", err))
			}
		}()

		loadnetwork.WaitForNodeMapperReady(libP2PNodeM, cfg.GlobalSys)

		consensusNode, err := pbft.NewPBFTNode(networkConn, libP2PNodeM, cfg.ConsensusNodeCfg, *lp)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to create a PBFT node: %w", err))
		}

		time.Sleep(nodeWaitingTime)

		consensusNode.Start()

	default:
		log.Fatal(fmt.Errorf("unsupported communication mode: %s", cfg.CommunicationMode))
	}
}
