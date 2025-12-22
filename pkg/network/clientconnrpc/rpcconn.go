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

func (r *RPCConn) HandleMessage(_ context.Context, req *rpcserver.HandleMessageRequest) (*rpcserver.HandleMessageResponse, error) {
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
	err := r.sendMessage(ctx, dest, msg)
	if err != nil {
		slog.ErrorContext(ctx, "SendMsg2Dest failed", "dest", dest, "err", err)
	}
}

// Close closes all the connections in the client pool.
func (r *RPCConn) Close() {
	time.Sleep(connCloseTime)

	// close all clients in the pool
	for _, c := range r.clientPool {
		_ = c.conn.Close()
	}
}

func (r *RPCConn) sendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) error {
	// if the dest node is me, add to the buffer directly
	if r.me == dest {
		return r.add2LocalBuffer(msg)
	}

	r.connLock.Lock()
	defer r.connLock.Unlock()

	if _, ok := r.info2Host[dest]; !ok {
		return fmt.Errorf("node %+v not exist in the p2p connection", dest)
	}
	// if there's no client, create one and reuse it.
	if _, ok := r.clientPool[dest]; !ok {
		conn, err := grpc.NewClient(r.info2Host[dest], grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(msgSizeLimit), grpc.MaxCallSendMsgSize(msgSizeLimit)))
		if err != nil {
			return fmt.Errorf("gRPC client fail to connect: %w", err)
		}

		r.clientPool[dest] = &clientConnection{conn, rpcserver.NewReplicaConnClient(conn)}
	}

	_, err := r.clientPool[dest].client.HandleMessage(ctx, &rpcserver.HandleMessageRequest{
		Msg:  msg,
		From: &rpcserver.NodePosition{ShardID: r.me.ShardID, NodeID: r.me.NodeID},
		To:   &rpcserver.NodePosition{ShardID: dest.ShardID, NodeID: dest.NodeID},
	})
	if err != nil {
		return fmt.Errorf("gRPC client fail to send message, type: %s, err: %w", msg.GetMsgType(), err)
	}

	return nil
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
