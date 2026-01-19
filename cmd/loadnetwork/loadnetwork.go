package loadnetwork

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/clientconnrpc"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/connlibp2p"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

const (
	nodeMapperCheckInterval = 2 * time.Second
	nodeMapperCheckTimes    = 30

	bootstrapAheadTime = 8 * time.Second
)

var (
	ipTablePath = flag.String("ip_table", "ip_table.json", "path to ip_table.json")

	errNodeMapNotReady = errors.New("node mapper is not ready")
)

func PrepareNetworkByCfg(cfg *config.Config, lp *config.LocalParams) (network.P2PConn, nodetopo.NodeMapper, error) {
	var (
		p2p   network.P2PConn
		nodeM nodetopo.NodeMapper
		err   error
	)

	switch cfg.CommunicationMode {
	case config.DirectConnMode:
		p2p, nodeM, err = loadDirectNetwork(cfg.GlobalSys, lp)
		if err != nil {
			return nil, nil, fmt.Errorf("load direct network: %w", err)
		}

		// Start gRPC server.
		go func() {
			if err = p2p.ListenStart(); err != nil {
				log.Fatal(fmt.Errorf("startServer: %w", err))
			}
		}()

	case config.LibP2PConnMode:
		p2p, nodeM = initLibP2PNetwork(cfg.NetworkCfg, lp)

		// Make sure that the supervisor (bootstrap) node starts first.
		if lp.ShardID != nodetopo.SupervisorShardID {
			time.Sleep(bootstrapAheadTime)
		}

		go func() {
			if err = p2p.ListenStart(); err != nil {
				log.Fatal(fmt.Errorf("startServer: %w", err))
			}
		}()

		if err = waitForNodeMapperReady(nodeM, cfg.GlobalSys); err != nil {
			return nil, nil, fmt.Errorf("wait for node mapper ready failed: %w", err)
		}

		slog.Info("node mapper ready, this node is to start")

	default:
		return nil, nil, fmt.Errorf("unknown communication Mode: %s", cfg.CommunicationMode)
	}

	return p2p, nodeM, nil
}

func loadDirectNetwork(cfg config.SystemCfg, lp *config.LocalParams) (network.P2PConn, nodetopo.NodeMapper, error) {
	info2Host, err := readIpTableFromFile(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("read ip table from the file failed: %w", err)
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

func initLibP2PNetwork(cfg config.NetworkCfg, lp *config.LocalParams) (network.P2PConn, nodetopo.NodeMapper) {
	slog.Info("local params is loaded successfully",
		"shard id", lp.ShardID, "node id", lp.NodeID, "wallet addr", lp.WalletAddr)
	meNode := nodetopo.NodeInfo{ShardID: lp.ShardID, NodeID: lp.NodeID}

	m := nodetopo.NewTopoGetter(make(map[int64]nodetopo.NodeInfo), make(map[int64][]nodetopo.NodeInfo))
	p2p := connlibp2p.NewLibP2PConn(cfg, meNode, m)

	return p2p, m
}

func waitForNodeMapperReady(nodeM nodetopo.NodeMapper, cfg config.SystemCfg) error {
	ticker := time.NewTicker(nodeMapperCheckInterval)
	defer ticker.Stop()

	err := retry.Do(func() error {
		err := checkNodeMapperReady(nodeM, cfg)
		if errors.Is(err, errNodeMapNotReady) {
			slog.Warn("node mapper is not ready, try again", "reason", err)
		} else if err != nil {
			slog.Error("failed to check node mapper ready, retry it", "error", err)
		}

		return err
	}, retry.Delay(nodeMapperCheckInterval), retry.Attempts(nodeMapperCheckTimes))

	return err
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

func checkNodeMapperReady(nodeM nodetopo.NodeMapper, cfg config.SystemCfg) error {
	allLeaders, err := nodeM.GetAllLeaders()
	if err != nil {
		return fmt.Errorf("failed to get all leaders in nodetopo: %w", err)
	}

	if len(allLeaders) != int(cfg.ShardNum) {
		return fmt.Errorf("actual leader number %d: %w", len(allLeaders), errNodeMapNotReady)
	}

	for shardID := range cfg.ShardNum {
		if _, err = nodeM.GetLeader(shardID); err != nil {
			return fmt.Errorf("failed to get leader of shard %d: %w", shardID, err)
		}

		nodeInfos, err := nodeM.GetNodesInShard(shardID)
		if err != nil {
			return fmt.Errorf("failed to get nodes in shard %d: %w", shardID, err)
		}

		if len(nodeInfos) != int(cfg.NodeNum) {
			return fmt.Errorf("actual node number %d in shard %d: %w", len(nodeInfos), shardID, errNodeMapNotReady)
		}
	}

	return nil
}
