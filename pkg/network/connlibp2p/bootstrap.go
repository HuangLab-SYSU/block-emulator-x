package connlibp2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

const (
	keyFile         = "./pkg/network/connlibp2p/bootstrap.key"
	secretKeyBitLen = 256

	reportConnTimeInterval = 30 * time.Second
)

// initBootstrap inits the network settings for supervisor node.
func (l *LibP2PConn) initBootstrap() (rErr error) {
	var cleanups []func()

	defer func() {
		if rErr != nil {
			for i := len(cleanups) - 1; i >= 0; i-- {
				cleanups[i]()
			}
		}
	}()

	ctx := context.Background()

	sk, err := loadOrInitBootstrapKey()
	if err != nil {
		return fmt.Errorf("failed to get private key for libp2p communication: %w", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(sk),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/12345"),
		libp2p.EnableRelayService(),
		libp2p.EnableNATService(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return fmt.Errorf("failed to start libp2p: %w", err)
	}

	relayInst, err := relay.New(h)
	if err != nil {
		return fmt.Errorf("failed to start relay service:%w", err)
	}

	cleanups = append(cleanups, func() { _ = relayInst.Close() })

	slog.Info("relay server running", "me", l.me)

	h.SetStreamHandler(RegisterProtocol, l.handleRegisterStream)
	h.SetStreamHandler(consensusMsgProtocol, l.handleMessage)

	// start DHT server
	kad, err := dht.New(ctx, h, dht.Mode(dht.ModeServer), dht.ProtocolPrefix(dhtProtocolPrefix))
	if err != nil {
		return fmt.Errorf("failed to start DHT service: %w", err)
	}

	cleanups = append(cleanups, func() { _ = kad.Close() })

	if err = kad.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to start bootstrap service: %w", err)
	}

	l.hostInst = h
	l.kadInst = kad

	slog.Info("successfully start bootstrap & relay & DHT service")

	return nil
}

func (l *LibP2PConn) reportLibConn() {
	reportTicker := time.NewTicker(reportConnTimeInterval)
	defer reportTicker.Stop()

	for range reportTicker.C {
		conns := l.hostInst.Network().Conns()
		slog.Info("bootstrap active connections", "count", len(conns))

		for _, c := range conns {
			slog.Info(
				"conn ->", "peer", c.RemotePeer().String(),
				"remoteAddr", c.RemoteMultiaddr().String(),
				"stat", c.Stat(),
			)
		}
	}
}

func (l *LibP2PConn) handleRegisterStream(s network.Stream) {
	defer func() { _ = s.Close() }()

	data, err := io.ReadAll(s)
	if err != nil {
		slog.Error("failed to read from register stream", "error", err)
		return
	}

	var info node2PeerIdInfo
	if err = json.Unmarshal(data, &info); err != nil {
		slog.Error("invalid node info JSON", "from", s.Conn().RemotePeer(), "error", err)

		_, err = s.Write([]byte("invalid json"))
		if err != nil {
			slog.Error("failed to send ACK", "to", s.Conn().RemotePeer(), "error", err)
		}

		if err = s.CloseWrite(); err != nil {
			slog.Error("failed to close write", "to", s.Conn().RemotePeer(), "error", err)
			_ = s.Reset()
		}

		return
	}

	// Check if the peerID matches.
	remotePeer := s.Conn().RemotePeer().String()
	if info.PeerID != remotePeer {
		slog.Error("failed to match peerID", "claimed is", info.PeerID, "actually is", remotePeer)

		_, err = s.Write([]byte("ailed to match peerID"))
		if err != nil {
			slog.Error("failed to send ACK", "to", remotePeer, "error", err)
		}

		if err = s.CloseWrite(); err != nil {
			slog.Error("failed to close write", "to", remotePeer, "error", err)

			_ = s.Reset()
		}

		return
	}

	// store Node2PeerIdInfo
	l.infoMapMux.Lock()

	if l.info2Host[info.ShardID] == nil {
		l.info2Host[info.ShardID] = make(map[int64]string)
	}

	l.info2Host[info.ShardID][info.NodeID] = info.PeerID
	l.infoMapMux.Unlock()

	// update the node topo map
	l.topoMux.Lock()
	defer l.topoMux.Unlock()

	err = l.NodeM.SetTopoGetter(l.info2Host)
	if err != nil {
		slog.Error("failed to set topogetter map", "error", err)
		return
	}

	slog.Info("registered node", "Shard:", info.ShardID, "Node:", info.NodeID, "Peer:", info.PeerID)

	_, err = s.Write([]byte("registered successfully"))
	if err != nil {
		slog.Error("failed to send ACK", "to", remotePeer, "error", err)
	}

	if err = s.CloseWrite(); err != nil {
		slog.Error("failed to close write", "to", remotePeer, "error", err)

		_ = s.Reset()
	}

	l.printRegisteredNodes()

	if err = l.broadcastNode2PeerIdInfos(); err != nil {
		slog.Warn("failed to broadcast ID map", "error", err)
	}
}

// broadcastNode2PeerIdInfos broadcasts the updated ID Map.
func (l *LibP2PConn) broadcastNode2PeerIdInfos() error {
	data, err := json.Marshal(l.info2Host)
	if err != nil {
		return fmt.Errorf("failed to marshal Node2PeerIdInfos: %w", err)
	}

	// Get all connected peers.
	conns := l.hostInst.Network().Conns()
	for _, conn := range conns {
		peerID := conn.RemotePeer()
		if peerID == l.hostInst.ID() {
			continue
		}

		go func(pid peer.ID) {
			ctx, cancel := context.WithTimeout(context.Background(), ctxTimeOut)
			defer cancel()

			s, err := l.hostInst.NewStream(ctx, pid, BroadcastIdMapProtocol)
			if err != nil {
				slog.Error("failed to open stream", "to", pid, "error", err)
				return
			}

			defer func() { _ = s.Close() }()

			if _, err = s.Write(data); err != nil {
				slog.Error("failed to send id map data", "to", pid, "error", err)
			} else {
				slog.Info("synced id map", "to", pid)
			}

			if err = s.CloseWrite(); err != nil {
				slog.Error("failed to close write", "to", pid, "error", err)

				_ = s.Reset()
			}
		}(peerID)
	}

	return nil
}

// printRegisteredNodes prints the registered nodes list.
func (l *LibP2PConn) printRegisteredNodes() {
	slog.Info("Total registered shards", "count", len(l.info2Host))

	for shardID, shardInfo := range l.info2Host {
		for nodeID, nodeInfo := range shardInfo {
			slog.Info("  - ", "Shard:", shardID, "Node:", nodeID, "Peer:", nodeInfo)
		}
	}
}

func loadOrInitBootstrapKey() (crypto.PrivKey, error) {
	sk, err := loadBootstrapKey()
	if err == nil {
		return sk, nil
	}

	slog.Info("loading bootstrap key failed, now creating a new bootstrap key ...")

	sk, err = generateBootstrapKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate bootstrap key: %w", err)
	}

	return sk, nil
}

// loadBootstrapKey loads or creates Ed25519 private key.
func loadBootstrapKey() (crypto.PrivKey, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file for getting the private key: %w", err)
	}

	sk, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	slog.Info("loaded existing private key", "from", keyFile)

	return sk, nil
}

// generateBootstrapKey creates secret key and save to file.
func generateBootstrapKey() (crypto.PrivKey, error) {
	sk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, secretKeyBitLen)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	data, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key: %w", err)
	}

	dir := filepath.Dir(keyFile)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create key dir: %w", err)
	}

	if err = os.WriteFile(keyFile, data, 0o600); err != nil {
		return nil, fmt.Errorf("failed to save private key to %s: %w", keyFile, err)
	}

	slog.Info("saved new private key", "to", keyFile)

	return sk, nil
}
