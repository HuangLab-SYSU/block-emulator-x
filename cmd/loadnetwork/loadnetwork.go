package loadnetwork

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/clientconnrpc"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/connlibp2p"
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

func InitNetworkAndNodeInfoWithLibP2PMode(lp *config.LocalParams) (network.P2PConn, error) {
	slog.Info("local params is loaded successfully",
		"shard id", lp.ShardID, "node id", lp.NodeID, "wallet addr", lp.WalletAddr)
	meNode := nodetopo.NodeInfo{ShardID: lp.ShardID, NodeID: lp.NodeID}

	// Set an RPC Connection as the P2P Connection.
	shardNodeInfo := make(map[int64][]nodetopo.NodeInfo)
	shardLeader := make(map[int64]nodetopo.NodeInfo)

	m := nodetopo.NewTopoGetter(shardLeader, shardNodeInfo)
	p2p := connlibp2p.NewLibP2PConn(meNode, m)

	return p2p, nil
}

func readIpTableFromFile(cfg config.SystemCfg) (map[nodetopo.NodeInfo]string, error) {
	// Read the contents of ip table (format: JSON)
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

func WaitForNodeMapperReady(nodeM nodetopo.NodeMapper, cfg config.SystemCfg) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(120 * time.Second)

	for {
		select {
		case <-ticker.C:
			// nodetopo 须符合条件： 1.分片Leader与数量正确；2.分片内节点数量正确
			ready := true
			allLeaders, err := nodeM.GetAllLeaders()
			if err != nil {
				slog.Error("failed to get all leaders in nodetopo")
			}
			if len(allLeaders) != int(cfg.ShardNum) {
				slog.Warn("count of shards is not correct")
				continue
			}
			for shardID := int64(0); shardID < cfg.ShardNum; shardID++ {
				if _, err := nodeM.GetLeader(shardID); err != nil {
					slog.Error("failed to get leader in", "shard", shardID)
					ready = false
					break
				}
				shardInfo, err := nodeM.GetNodesInShard(shardID)
				if err != nil {
					slog.Error("failed to get all nodes in", "shard", shardID)
					ready = false
					break
				}
				if len(shardInfo) != int(cfg.NodeNum) {
					slog.Warn("count of nodes is not correct in", "shard", shardID)
					ready = false
					break
				}
			}

			if ready {
				slog.Info("the NodeMapper is ready")
				return
			}

		case <-timeout:
			log.Fatal("timeout waiting for NodeMapper to be ready")
		}
	}
}
