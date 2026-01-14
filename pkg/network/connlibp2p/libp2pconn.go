package connlibp2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
	golog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	_ "github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
)

const (
	msgBufferSize          = 1 << 20
	bootstrapPeerID        = "12D3KooWR6siPMZ2sMFKbgwaJFwQfnKczuPZnxHfyy1dHTzZSAUY"
	ProtocolId             = "/myapp/chat/1.0.0"
	RegisterProtocol       = "/myapp/register/1.0.0"
	BroadcastIdMapProtocol = "/myapp/broadcastIdMap/1.0.0"
)

type LibP2PConn struct {
	mux        sync.Mutex
	infoMapMux sync.RWMutex

	topoMux sync.Mutex
	NodeM   nodetopo.NodeMapper

	me        nodetopo.NodeInfo
	info2Host map[int64]map[int64]string
	msgBuffer chan *rpcserver.WrappedMsg

	once       sync.Once
	hostInst   host.Host
	kadInst    *dht.IpfsDHT
	ctx        context.Context
	cancelFunc context.CancelFunc
	initErr    error
}

type node2PeerIdInfo struct {
	ShardID int64
	NodeID  int64
	PeerID  string
}

func init() {
	golog.SetAllLoggers(golog.LevelError)
}

func NewLibP2PConn(me nodetopo.NodeInfo, nodeM nodetopo.NodeMapper) *LibP2PConn {
	info2Host := make(map[int64]map[int64]string)
	info2Host[nodetopo.SupervisorShardID] = map[int64]string{
		0: bootstrapPeerID,
	}
	return &LibP2PConn{
		me:        me,
		info2Host: info2Host,
		msgBuffer: make(chan *rpcserver.WrappedMsg, msgBufferSize),
		NodeM:     nodeM,
	}
}

func (l *LibP2PConn) ListenStart() error {
	if l.me.ShardID == 0x7fffffff && l.me.NodeID == 0 {
		return l.initBootstrap()
	}
	return l.initLibP2PConnect()
}

func (l *LibP2PConn) initLibP2PConnect() error {
	// hostAddr := "/ip4/" + params.RelayIP + "/tcp/" + strconv.Itoa(params.RelayPort) + "/p2p/" + params.RelayID
	hostAddr := "/ip4/127.0.0.1/tcp/12345/p2p/" + bootstrapPeerID
	l.once.Do(func() {
		l.ctx, l.cancelFunc = context.WithCancel(context.Background())

		// decode the relay address
		bootAddr, err := multiaddr.NewMultiaddr(hostAddr)
		if err != nil {
			l.initErr = fmt.Errorf("invalid bootstrap addr: %w", err)
			return
		}
		bootInfo, err := peer.AddrInfoFromP2pAddr(bootAddr)
		if err != nil {
			l.initErr = fmt.Errorf("parse p2p addr failed: %w", err)
			return
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
			l.initErr = fmt.Errorf("libp2p.New failed: %w", err)
			return
		}

		// Try to connect the bootstrap node for 30 times.
		var connectErr error
		maxRetries := 30
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if err := h.Connect(l.ctx, *bootInfo); err == nil {
				slog.Info("connected to bootstrap relay successfully")
				connectErr = nil
				break
			} else {
				connectErr = fmt.Errorf("connect attempt %d failed: %w", attempt, err)
				log.Println(connectErr)
				if attempt < maxRetries {
					time.Sleep(2 * time.Second)
				}
			}
		}
		if connectErr != nil {
			l.initErr = fmt.Errorf("failed to connect to bootstrap relay after %d attempts: %w", maxRetries, connectErr)
			return
		}

		// Start private DHT.
		kad, err := dht.New(l.ctx, h,
			dht.Mode(dht.ModeAuto),
			dht.ProtocolPrefix("/myapp/kad/1.0.0"),
			dht.BootstrapPeers(*bootInfo),
		)
		if err != nil {
			l.initErr = fmt.Errorf("DHT new failed: %v", err)
			return
		}
		if err = kad.Bootstrap(l.ctx); err != nil {
			l.initErr = fmt.Errorf("DHT bootstrap failed: %v", err)
			return
		}

		// Start mDNS.
		mdnsService := mdns.NewMdnsService(h, "myapp.local", &mdnsNotifee{h: h})
		if err = mdnsService.Start(); err != nil {
			log.Printf("mDNS start failed (non-fatal): %v", err)
		}

		// Set stream handler to receive message from other client nodes.
		h.SetStreamHandler(ProtocolId, l.handleMessage)
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
				addr := conn.RemoteMultiaddr().String()

				mu.Lock()
				defer mu.Unlock()

				if strings.Contains(addr, "/p2p-circuit") {
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

		slog.Info("client node started libp2p communication", "peer id", h.ID())

		// upload(or register) node info to supervisor node
		for {
			err = l.registerNodeInfo(l.me)
			if err != nil {
				slog.Error("failed to register node info", "error", err)
			} else {
				slog.Info("node registered with bootstrap successfully")
				break
			}
			time.Sleep(2 * time.Second)
		}
	})

	return l.initErr
}

func (l *LibP2PConn) DrainMsgBuffer() []*rpcserver.WrappedMsg {
	l.mux.Lock()
	defer l.mux.Unlock()

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

func (l *LibP2PConn) GroupBroadcastMessage(ctx context.Context, group []nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	wg := &sync.WaitGroup{}
	wg.Add(len(group))
	// Broadcast to all nodes in this group.
	for _, node := range group {
		go func(nif nodetopo.NodeInfo) {
			defer wg.Done()

			err := l.sendMessage(ctx, nif, msg)
			if err != nil {
				slog.ErrorContext(ctx, "sub-goroutine: broadcast", "err", err)
			}
		}(node)
	}

	wg.Wait()
}

// Close closes all the connections in the client pool.
func (l *LibP2PConn) Close() {
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

	ctxWithTimeout, cancel := context.WithTimeout(l.ctx, 10*time.Second)
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
	ack, err := io.ReadAll(s)
	if err != nil {
		l.initErr = fmt.Errorf("failed to read ACK: %w", err)
		return l.initErr
	}
	fmt.Printf("Bootstrap reply: %s\n", string(ack))

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
	err = l.add2LocalBuffer(&msg)
	if err != nil {
		slog.Error("failed to add message to local buffer")
	}
}

func (l *LibP2PConn) handleIdMapMessage(s network.Stream) {
	defer func() { _ = s.Close() }()
	data := make([]byte, 4096)
	n, err := s.Read(data)
	if err != nil {
		slog.Error("failed to read id map from message stream")
		return
	}

	var newMap map[int64]map[int64]string
	if err = json.Unmarshal(data[:n], &newMap); err != nil {
		slog.Error("failed to unmarshal id map from message stream")
		return
	}

	// Update the ID map.
	l.infoMapMux.Lock()
	l.info2Host = newMap
	l.infoMapMux.Unlock()

	// Update the nodetopo map.
	l.topoMux.Lock()
	err = l.NodeM.SetTopoGetter(l.info2Host)
	l.topoMux.Unlock()
	if err != nil {
		slog.Error("failed to set topogetter map", "error", err)
		return
	}

}

func (l *LibP2PConn) sendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) error {
	// if the dest node is me, add the message to the buffer directly
	if l.me == dest {
		return l.add2LocalBuffer(msg)
	}
	peerID := l.info2Host[dest.ShardID][dest.NodeID]
	targetID, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	// create stream
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	s, err := l.hostInst.NewStream(ctxWithTimeout, targetID, ProtocolId)
	if err != nil {
		return fmt.Errorf("p2pDial failed to open stream to %s: %w", peerID, err)
	}
	defer func() { _ = s.Close() }()

	// send data
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	if _, err := s.Write(data); err != nil {
		return fmt.Errorf("p2pDial write error to %s: %w", peerID, err)
	}

	if err := s.CloseWrite(); err != nil {
		slog.Warn("failed to close write", "to", peerID, "error", err)
		_ = s.Reset()
	}

	return nil

}

func (l *LibP2PConn) add2LocalBuffer(msg *rpcserver.WrappedMsg) error {
	l.mux.Lock()
	defer l.mux.Unlock()

	select {
	case l.msgBuffer <- msg:
	default:
		return fmt.Errorf("message buffer is full or closed")
	}

	return nil
}

type mdnsNotifee struct {
	h host.Host
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	slog.Info("mDNS found", "peer", pi.ID)
	n.h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)

	if n.h.Network().Connectedness(pi.ID) != network.Connected {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := n.h.Connect(ctx, pi); err != nil {
				slog.Error("mDNS failed to auto-connect", "to", pi.ID, "error", err)
			} else {
				slog.Info("mDNS successfully auto-connected", "to", pi.ID)
			}
		}()
	}
}
