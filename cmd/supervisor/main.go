package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/supervisor"
)

var (
	configPath  = flag.String("config", "config.yaml", "path to config file")
	ipTablePath = flag.String("ip_table", "ip_table.yaml", "path to ip_table.yaml")
)

func main() {
	flag.Parse()

	ipTable, err := readIpTableFromFile(*ipTablePath)
	if err != nil {
		log.Fatal(fmt.Errorf("getNetworkAndNodeTopo: %w", err))
	}

	p2p, nodeM, err := getNetworkAndNodeTopo(ipTable)
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

func getNetworkAndNodeTopo(info2Host map[nodetopo.NodeInfo]string) (*network.P2PConn, nodetopo.NodeMapper, error) {
	lp, err := config.LoadLocalParams()
	if err != nil {
		return nil, nil, fmt.Errorf("config.LoadLocalParams: %w", err)
	}

	meNode := nodetopo.NodeInfo{ShardID: lp.ShardID, NodeID: lp.NodeID}

	p2p := network.NewP2PConn(meNode, info2Host)
	shardNodeInfo := make(map[int64][]nodetopo.NodeInfo)
	shardLeader := make(map[int64]nodetopo.NodeInfo)

	for node := range info2Host {
		shardID := node.ShardID

		shardNodeInfo[shardID] = append(shardNodeInfo[shardID], node)
		if node.NodeID == 0 {
			shardLeader[shardID] = node
		}
	}

	m := nodetopo.NewTopoGetter(shardLeader, shardNodeInfo)

	return p2p, m, nil
}

func readIpTableFromFile(ipFilePath string) (map[nodetopo.NodeInfo]string, error) {
	panic("implement me")
}
