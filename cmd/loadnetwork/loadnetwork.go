package loadnetwork

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/clientconnrpc"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

var ipTablePath = flag.String("ip_table", "ip_table.json", "path to ip_table.json")

func GetNetworkAndNodeInfo(cfg config.SystemCfg, lp *config.LocalParams) (network.P2PConn, nodetopo.NodeMapper, error) {
	info2Host, err := readIpTableFromFile(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("getNetworkAndNodeTopo: %w", err)
	}

	slog.Info("local params is loaded successfully",
		"shard id", lp.ShardID, "node id", lp.NodeID, "wallet addr", lp.WalletAddr, "ip addr",
		info2Host[nodetopo.NodeInfo{ShardID: lp.ShardID, NodeID: lp.NodeID}])
	meNode := nodetopo.NodeInfo{ShardID: lp.ShardID, NodeID: lp.NodeID}

	// Set an RPC Connection as the P2P Connection.
	p2p := clientconnrpc.NewRPCConn(meNode, info2Host)
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

func readIpTableFromFile(cfg config.SystemCfg) (map[nodetopo.NodeInfo]string, error) {
	// Read the contents of ip table (format: json)
	file, err := os.ReadFile(*ipTablePath)
	if err != nil {
		return nil, fmt.Errorf("readIpTableFromFile: %w", err)
	}
	// Create a map to store the IP addresses
	var ipMap map[int64]map[int64]string
	// Unmarshal the JSON data into the map
	if err = json.Unmarshal(file, &ipMap); err != nil {
		return nil, fmt.Errorf("readIpTableFromFile: %w", err)
	}

	ret := make(map[nodetopo.NodeInfo]string)

	for shardID, shardInfoMap := range ipMap {
		// Skip the invalid shardID
		if shardID != nodetopo.SupervisorShardID && (shardID >= cfg.ShardNum || shardID < 0) {
			continue
		}

		for nodeID, ip := range shardInfoMap {
			if nodeID >= cfg.NodeNum || nodeID < 0 {
				continue
			}

			ret[nodetopo.NodeInfo{ShardID: shardID, NodeID: nodeID}] = ip
		}
	}

	return ret, nil
}
