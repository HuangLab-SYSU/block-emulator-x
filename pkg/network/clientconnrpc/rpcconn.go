package clientconnrpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

const (
	msgBufferSize = 1 << 20
	msgSizeLimit  = 100 * 1 << 20
	rpcTimeout    = 10 * time.Second
	connCloseTime = 10 * time.Second
)

type clientConnection struct {
	conn   *grpc.ClientConn
	client rpcserver.ReplicaConnClient
}

// RPCConn implements the interfaces 'P2PConn'.
// It is based on the fixed IPs defined in the map `info2Host`.
// RPCConn also implements the interface 'ReplicaConnServer', which is used to start a gRPC server, thus
// it can listen messages from other nodes by gRPC.
type RPCConn struct {
	mux sync.Mutex

	me        nodetopo.NodeInfo            // the NodeInfo of this node
	info2Host map[nodetopo.NodeInfo]string // the mapper from the NodeInfos to the node hosts
	msgBuffer chan *rpcserver.WrappedMsg   // the buffer of a set of WrappedMsg

	connLock   sync.Mutex
	clientPool map[nodetopo.NodeInfo]*clientConnection

	rpcserver.UnimplementedReplicaConnServer
}

func NewRPCConn(me nodetopo.NodeInfo, info2Host map[nodetopo.NodeInfo]string) *RPCConn {
	return &RPCConn{
		me:         me,
		info2Host:  info2Host,
		clientPool: make(map[nodetopo.NodeInfo]*clientConnection),
		msgBuffer:  make(chan *rpcserver.WrappedMsg, msgBufferSize),
	}
}

func (r *RPCConn) ListenStart() error {
	listenAddr := r.info2Host[r.me]

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	s := grpc.NewServer(grpc.MaxRecvMsgSize(msgSizeLimit), grpc.MaxSendMsgSize(msgSizeLimit))

	rpcserver.RegisterReplicaConnServer(s, r)

	slog.Info("gRPC P2P server listening", "addr", listenAddr)

	return s.Serve(lis)
}

func (r *RPCConn) HandleMessage(
	_ context.Context,
	req *rpcserver.HandleMessageRequest,
) (*rpcserver.HandleMessageResponse, error) {
	if err := r.add2LocalBuffer(req.GetMsg()); err != nil {
		return nil, fmt.Errorf("add message to buffer failed: %w", err)
	}

	slog.Debug("handle message to local buffer", "from", req.GetFrom(), "to", req.GetTo())

	return &rpcserver.HandleMessageResponse{Ack: true}, nil
}

func (r *RPCConn) DrainMsgBuffer() []*rpcserver.WrappedMsg {
	r.mux.Lock()
	defer r.mux.Unlock()

	ret := make([]*rpcserver.WrappedMsg, 0, len(r.msgBuffer))

	for {
		select {
		case msg := <-r.msgBuffer:
			ret = append(ret, msg)
		default:
			return ret
		}
	}
}

func (r *RPCConn) SendMsg2Dest(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	start := time.Now()

	err := r.sendMessage(ctx, dest, msg)
	if err != nil {
		slog.ErrorContext(ctx, "SendMsg2Dest failed",
			"dest", dest, "type", msg.GetMsgType(), "payload_bytes", len(msg.GetPayload()),
			"elapsed", time.Since(start), "err", err)
	}
}

// Close closes all the connections in the client pool.
func (r *RPCConn) Close() {
	time.Sleep(connCloseTime)

	r.connLock.Lock()
	clients := r.clientPool
	r.clientPool = nil
	r.connLock.Unlock()

	for dest, c := range clients {
		if err := c.conn.Close(); err != nil {
			slog.Warn("close gRPC client failed", "dest", dest, "err", err)
		}
	}
}

func (r *RPCConn) sendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) error {
	// if the dest node is me, add to the buffer directly
	if r.me == dest {
		return r.add2LocalBuffer(msg)
	}

	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	client, err := r.getClient(dest)
	if err != nil {
		return err
	}

	request := &rpcserver.HandleMessageRequest{
		Msg:  msg,
		From: &rpcserver.NodePosition{ShardID: r.me.ShardID, NodeID: r.me.NodeID},
		To:   &rpcserver.NodePosition{ShardID: dest.ShardID, NodeID: dest.NodeID},
	}
	if _, err := client.HandleMessage(ctx, request); err != nil {
		return fmt.Errorf("gRPC client fail to send message, type: %s, err: %w", msg.GetMsgType(), err)
	}

	return nil
}

// getClient holds the pool lock only while looking up or creating a connection.
func (r *RPCConn) getClient(dest nodetopo.NodeInfo) (rpcserver.ReplicaConnClient, error) {
	r.connLock.Lock()
	defer r.connLock.Unlock()

	if r.clientPool == nil {
		return nil, fmt.Errorf("gRPC client pool is closed")
	}

	if c, ok := r.clientPool[dest]; ok {
		return c.client, nil
	}

	host, ok := r.info2Host[dest]
	if !ok {
		return nil, fmt.Errorf("node %+v not exist in the p2p connection", dest)
	}

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(msgSizeLimit), grpc.MaxCallSendMsgSize(msgSizeLimit)))
	if err != nil {
		return nil, fmt.Errorf("gRPC client fail to connect: %w", err)
	}

	client := rpcserver.NewReplicaConnClient(conn)
	r.clientPool[dest] = &clientConnection{conn: conn, client: client}

	return client, nil
}

func (r *RPCConn) add2LocalBuffer(msg *rpcserver.WrappedMsg) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	select {
	case r.msgBuffer <- msg:
	default:
		return fmt.Errorf("message buffer is full or closed")
	}

	return nil
}
