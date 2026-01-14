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

const keyFile = "./pkg/network/connlibp2p/bootstrap.key"

// getBootstrapKey loads or creates Ed25519 private key.
func (l *LibP2PConn) getBootstrapKey() (crypto.PrivKey, error) {

	dir := filepath.Dir(keyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create key dir: %w", err)
	}

	// Load secret key from file.
	if data, err := os.ReadFile(keyFile); err == nil {
		sk, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			slog.Error("failed to unmarshal existing key, will generate new one", "error", err)
		} else {
			slog.Info("loaded existing private key", "from", keyFile)
			return sk, nil
		}
	} else {
		slog.Error("failed to read file for getting the private key, will generate new one", "error", err)
	}

	// Create secret key and save to file.
	sk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	data, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key: %w", err)
	}

	if err = os.WriteFile(keyFile, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key to %s: %w", keyFile, err)
	}
	slog.Info("saved new private key", "to", keyFile)

	return sk, nil
}

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

	sk, err := l.getBootstrapKey()
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
	slog.Info("relay server running", "on", l.me)

	h.SetStreamHandler(RegisterProtocol, l.handleRegisterStream)
	h.SetStreamHandler(ProtocolId, l.handleMessage)

	// start DHT server
	kad, err := dht.New(ctx, h,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix("/myapp/kad/1.0.0"),
	)
	if err != nil {
		return fmt.Errorf("failed to start DHT service:%w", err)
	}
	cleanups = append(cleanups, func() { _ = kad.Close() })
	if err = kad.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to start bootstrap service:%w", err)
	}

	l.hostInst = h
	l.kadInst = kad

	slog.Info("successfully start bootstrap & relay & DHT service")

	go func() {
		for {
			conns := h.Network().Conns()
			slog.Info("bootstrap active connections", "count", len(conns))
			for _, c := range conns {
				slog.Info("  conn -> ", "peer=", c.RemotePeer().String(), "remoteAddr=", c.RemoteMultiaddr().String(), "stat=", c.Stat())
			}
			time.Sleep(30 * time.Second)
		}
	}()

	select {}
}

func (l *LibP2PConn) handleRegisterStream(s network.Stream) {
	defer func(s network.Stream) {
		err := s.Close()
		if err != nil {
		}
	}(s)
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
			err = s.Reset()
			if err != nil {
				return
			}
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
	err = l.NodeM.SetTopoGetter(l.info2Host)
	l.topoMux.Unlock()
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
	err = l.broadcastNode2PeerIdInfos()
	if err != nil {
		slog.Warn("failed to broadcast ID map", "to", "error", err)
		return
	}
}

// broadcastNode2PeerIdInfos broadcasts the updated ID Map.
func (l *LibP2PConn) broadcastNode2PeerIdInfos() error {
	data, err := json.Marshal(l.info2Host)

	if err != nil {
		return fmt.Errorf("failed to marshal Node2PeerIdInfos: %w", err)
	}

	// get all connected peers
	conns := l.hostInst.Network().Conns()
	for _, conn := range conns {
		peerID := conn.RemotePeer()
		if peerID == l.hostInst.ID() {
			continue
		}
		go func(pid peer.ID) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			s, err := l.hostInst.NewStream(ctx, pid, BroadcastIdMapProtocol)
			if err != nil {
				slog.Error("failed to open stream", "to", pid, "error", err)
				return

			}
			defer func(s network.Stream) {
				if err = s.Close(); err != nil {
				}
			}(s)

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
