package connlibp2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	golog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"

	_ "github.com/libp2p/go-libp2p/p2p/host/autorelay"
)

const (
	msgBufferSize          = 1 << 20
	bootstrapPeerID        = "12D3KooWR6siPMZ2sMFKbgwaJFwQfnKczuPZnxHfyy1dHTzZSAUY"
	consensusMsgProtocol   = "/myapp/chat/1.0.0"
	RegisterProtocol       = "/myapp/register/1.0.0"
	BroadcastIdMapProtocol = "/myapp/broadcastIdMap/1.0.0"
	multiAddrSubstr        = "/p2p-circuit"
	mdnsServerName         = "myapp.local"
	dhtProtocolPrefix      = "/myapp/kad/1.0.0"

	ctxTimeOut    = 10 * time.Second
	retryInterval = 2 * time.Second
	maxRetries    = 30
)

type LibP2PConn struct {
	buffMux sync.Mutex

	infoMapMux, topoMux sync.Mutex
	NodeM               nodetopo.NodeMapper

	me        nodetopo.NodeInfo
	info2Host map[int64]map[int64]string
	msgBuffer chan *rpcserver.WrappedMsg

	hostInst host.Host
	kadInst  *dht.IpfsDHT
}

type node2PeerIdInfo struct {
	ShardID int64
	NodeID  int64
	PeerID  string
}

func NewLibP2PConn(me nodetopo.NodeInfo, nodeM nodetopo.NodeMapper) *LibP2PConn {
	golog.SetAllLoggers(golog.LevelError)

	info2Host := make(map[int64]map[int64]string)
	info2Host[nodetopo.SupervisorShardID] = map[int64]string{0: bootstrapPeerID}

	return &LibP2PConn{
		me:        me,
		info2Host: info2Host,
		msgBuffer: make(chan *rpcserver.WrappedMsg, msgBufferSize),
		NodeM:     nodeM,
	}
}

func (l *LibP2PConn) ListenStart() error {
	if l.me.ShardID != nodetopo.SupervisorShardID {
		return l.initLibP2PConnect()
	}

	if err := l.initBootstrap(); err != nil {
		return fmt.Errorf("failed to init bootstrap: %w", err)
	}

	l.reportLibConn()

	return nil
}

func (l *LibP2PConn) DrainMsgBuffer() []*rpcserver.WrappedMsg {
	l.buffMux.Lock()
	defer l.buffMux.Unlock()

	ret := make([]*rpcserver.WrappedMsg, 0)

	for {
		select {
		case msg := <-l.msgBuffer:
			ret = append(ret, msg)
		default:
			return ret
		}
	}
}

func (l *LibP2PConn) SendMsg2Dest(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	err := l.sendMessage(ctx, dest, msg)
	if err != nil {
		slog.ErrorContext(ctx, "SendMessage failed", "dest", dest, "err", err)
	}
}

// Close closes all the connections in the client pool.
func (l *LibP2PConn) Close() {
}

func (l *LibP2PConn) initLibP2PConnect() error {
	hostAddr := "/ip4/127.0.0.1/tcp/12345/p2p/" + bootstrapPeerID

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// Decode the relay address.
	bootAddr, err := multiaddr.NewMultiaddr(hostAddr)
	if err != nil {
		return fmt.Errorf("invalid bootstrap addr: %w", err)
	}

	bootInfo, err := peer.AddrInfoFromP2pAddr(bootAddr)
	if err != nil {
		return fmt.Errorf("parse p2p addr failed: %w", err)
	}

	// Create host.
	h, err := libp2p.New(
		libp2p.EnableAutoNATv2(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableHolePunching(),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*bootInfo}),
	)
	if err != nil {
		return fmt.Errorf("libp2p.New failed: %w", err)
	}

	// Try to connect the bootstrap node for 30 times.
	if err = retry.Do(func() error { return h.Connect(ctx, *bootInfo) },
		retry.Delay(retryInterval), retry.Attempts(maxRetries)); err != nil {
		return fmt.Errorf("failed to connect to bootstrap relay after: %w", err)
	}

	// Start private DHT.
	kad, err := dht.New(ctx, h, dht.ProtocolPrefix(dhtProtocolPrefix), dht.BootstrapPeers(*bootInfo))
	if err != nil {
		return fmt.Errorf("DHT new failed: %w", err)
	}

	if err = kad.Bootstrap(ctx); err != nil {
		return fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	// Start mDNS.
	mdnsService := mdns.NewMdnsService(h, mdnsServerName, &mdnsNotifee{h: h})
	if err = mdnsService.Start(); err != nil {
		return fmt.Errorf("mDNS start failed (non-fatal): %w", err)
	}

	// Set stream handler to receive message from other client nodes.
	h.SetStreamHandler(consensusMsgProtocol, l.handleMessage)
	// Set stream handler to receive id map message from supervisor nodes.
	h.SetStreamHandler(BroadcastIdMapProtocol, l.handleIdMapMessage)

	// Record the connection type.
	var (
		mu        sync.Mutex
		hasDirect = make(map[peer.ID]bool)
		hasRelay  = make(map[peer.ID]bool)
	)

	h.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			pid := conn.RemotePeer()
			multiAddr := conn.RemoteMultiaddr().String()

			mu.Lock()
			defer mu.Unlock()

			if strings.Contains(multiAddr, multiAddrSubstr) {
				hasRelay[pid] = true
				slog.Info("via RELAY connected", "to", pid)
			} else {
				hasDirect[pid] = true
				slog.Info("via DIRECT connected", "to", pid)

				if hasRelay[pid] {
					slog.Info("hole punching successfully", "for", pid)
				}
			}
		},
	})

	l.hostInst = h
	l.kadInst = kad

	slog.Info("client node started lib p2p communication", "peer id", h.ID())

	// Upload (or register) node info to supervisor node.
	if err = retry.Do(func() error { return l.registerNodeInfo(l.me) },
		retry.Delay(retryInterval), retry.Attempts(maxRetries)); err != nil {
		return fmt.Errorf("failed to register node info: %w", err)
	}

	slog.Info("node registered with bootstrap successfully")

	return nil
}

func (l *LibP2PConn) registerNodeInfo(me nodetopo.NodeInfo) error {
	if l.hostInst == nil || l.kadInst == nil {
		return fmt.Errorf("libp2p not initialized")
	}

	relayPeerID, err := peer.Decode(bootstrapPeerID)
	if err != nil {
		return fmt.Errorf("invalid relay peer ID: %w", err)
	}

	node2Host := node2PeerIdInfo{
		ShardID: me.ShardID,
		NodeID:  me.NodeID,
		PeerID:  l.hostInst.ID().String(),
	}

	data, err := json.Marshal(node2Host)
	if err != nil {
		return fmt.Errorf("marshal node info failed: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), ctxTimeOut)
	defer cancel()

	// Create stream.
	s, err := l.hostInst.NewStream(
		network.WithAllowLimitedConn(ctxWithTimeout, "register"),
		relayPeerID,
		RegisterProtocol,
	)
	if err != nil {
		return fmt.Errorf("failed to open register stream: %w", err)
	}

	defer func() { _ = s.Close() }()

	// Send data to supervisor.
	_, err = s.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send node info: %w", err)
	}

	if err = s.CloseWrite(); err != nil {
		slog.Error("failed to close write", "to", relayPeerID, "error", err)

		_ = s.Reset()
	}

	// Read ACK from supervisor.
	if _, err = io.ReadAll(s); err != nil {
		return fmt.Errorf("failed to read ACK: %w", err)
	}

	return nil
}

func (l *LibP2PConn) handleMessage(s network.Stream) {
	defer func() { _ = s.Close() }()

	data, err := io.ReadAll(s)
	if err != nil {
		slog.Error("failed to read stream", "from", string(s.Conn().RemotePeer()))
	}

	var msg rpcserver.WrappedMsg
	if err = proto.Unmarshal(data, &msg); err != nil {
		slog.Error("failed to unmarshal protobuf message",
			"from", s.Conn().RemotePeer().String(),
			"error", err,
			"data_len", len(data))

		return
	}

	slog.Info("received message", "from", s.Conn().RemotePeer())

	if err = l.add2LocalBuffer(&msg); err != nil {
		slog.Error("failed to add message to local buffer")
	}
}

func (l *LibP2PConn) handleIdMapMessage(s network.Stream) {
	defer func() { _ = s.Close() }()

	data, err := io.ReadAll(s)
	if err != nil {
		slog.Error("failed to read id map from message stream")
		return
	}

	var newMap map[int64]map[int64]string
	if err = json.Unmarshal(data, &newMap); err != nil {
		slog.Error("failed to unmarshal id map from message stream")
		return
	}

	// Update the ID map.
	l.infoMapMux.Lock()
	l.info2Host = newMap
	l.infoMapMux.Unlock()

	// Update the nodetopo map.
	l.topoMux.Lock()
	defer l.topoMux.Unlock()

	err = l.NodeM.SetTopoGetter(l.info2Host)
	if err != nil {
		slog.Error("failed to set topogetter map", "error", err)
	}
}

func (l *LibP2PConn) sendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) error {
	// If the dest node is me, add the message to the buffer directly
	if l.me == dest {
		return l.add2LocalBuffer(msg)
	}

	peerID := l.info2Host[dest.ShardID][dest.NodeID]

	targetID, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	// Create stream.
	ctxWithTimeout, cancel := context.WithTimeout(ctx, ctxTimeOut)
	defer cancel()

	s, err := l.hostInst.NewStream(ctxWithTimeout, targetID, consensusMsgProtocol)
	if err != nil {
		return fmt.Errorf("p2pDial failed to open stream to %s: %w", peerID, err)
	}

	defer func() { _ = s.Close() }()

	// Send data.
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if _, err = s.Write(data); err != nil {
		return fmt.Errorf("p2pDial write error to %s: %w", peerID, err)
	}

	if err = s.CloseWrite(); err != nil {
		slog.Warn("failed to close write", "to", peerID, "error", err)

		_ = s.Reset()
	}

	return nil
}

func (l *LibP2PConn) add2LocalBuffer(msg *rpcserver.WrappedMsg) error {
	l.buffMux.Lock()
	defer l.buffMux.Unlock()

	select {
	case l.msgBuffer <- msg:
	default:
		return fmt.Errorf("message buffer is full or closed")
	}

	return nil
}
