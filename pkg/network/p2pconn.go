package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

const msgBufferSize = 1 << 20

type clientConnection struct {
	conn   *grpc.ClientConn
	client rpcserver.ReplicaConnClient
}

type P2PConn struct {
	mux sync.Mutex

	me        nodetopo.NodeInfo
	info2Host map[nodetopo.NodeInfo]string
	msgBuffer chan *rpcserver.WrappedMsg

	connLock   sync.Mutex
	clientPool map[nodetopo.NodeInfo]*clientConnection

	rpcserver.UnimplementedReplicaConnServer
}

func NewP2PConn(me nodetopo.NodeInfo, info2Host map[nodetopo.NodeInfo]string) *P2PConn {
	return &P2PConn{
		me:         me,
		info2Host:  info2Host,
		clientPool: make(map[nodetopo.NodeInfo]*clientConnection),
		msgBuffer:  make(chan *rpcserver.WrappedMsg, msgBufferSize),
	}
}

func (p *P2PConn) StartServer() error {
	listenAddr := p.info2Host[p.me]

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	s := grpc.NewServer()

	rpcserver.RegisterReplicaConnServer(s, p)

	slog.Info("gRPC P2P server listening", "addr", listenAddr)

	return s.Serve(lis)
}

func (p *P2PConn) HandleMessage(_ context.Context, req *rpcserver.HandleMessageRequest) (*rpcserver.HandleMessageResponse, error) {
	if err := p.add2LocalBuffer(req.GetMsg()); err != nil {
		return nil, fmt.Errorf("add message to buffer failed: %w", err)
	}

	slog.Debug("handle message to local buffer", "from", req.GetFrom(), "to", req.GetTo())

	return &rpcserver.HandleMessageResponse{Ack: true}, nil
}

func (p *P2PConn) GetMeNodeInfo() nodetopo.NodeInfo {
	return p.me
}

func (p *P2PConn) ReadMsgBuffer() []*rpcserver.WrappedMsg {
	p.mux.Lock()
	defer p.mux.Unlock()

	ret := make([]*rpcserver.WrappedMsg, 0)

	for {
		select {
		case msg := <-p.msgBuffer:
			ret = append(ret, msg)
		default:
			return ret
		}
	}
}

func (p *P2PConn) SendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	err := p.sendMessage(ctx, dest, msg)
	if err != nil {
		slog.ErrorContext(ctx, "SendMessage failed", "dest", dest, "err", err)
	}
}

func (p *P2PConn) MSendDifferentMessages(ctx context.Context, node2Msg map[nodetopo.NodeInfo]*rpcserver.WrappedMsg) {
	wg := sync.WaitGroup{}
	wg.Add(len(node2Msg))

	for node, msg := range node2Msg {
		go func(node nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
			defer wg.Done()

			err := p.sendMessage(ctx, node, msg)
			if err != nil {
				slog.ErrorContext(ctx, "sub-goroutine: MSendDifferentMessages", "err", err)
			}
		}(node, msg)
	}

	wg.Wait()
}

func (p *P2PConn) GroupBroadcastMessage(ctx context.Context, group []nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	wg := &sync.WaitGroup{}
	wg.Add(len(group))
	// broadcast to all nodes in this group
	for _, node := range group {
		go func(nif nodetopo.NodeInfo) {
			defer wg.Done()

			err := p.sendMessage(ctx, nif, msg)
			if err != nil {
				slog.ErrorContext(ctx, "sub-goroutine: broadcast", "err", err)
			}
		}(node)
	}

	wg.Wait()
}

// Close closes all the connections in the client pool.
func (p *P2PConn) Close() {
	// close all clients in the pool
	for _, c := range p.clientPool {
		_ = c.conn.Close()
	}
}

func (p *P2PConn) sendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) error {
	// if the dest node is me, add to the buffer directly
	if p.me == dest {
		return p.add2LocalBuffer(msg)
	}

	p.connLock.Lock()
	defer p.connLock.Unlock()

	if _, ok := p.info2Host[dest]; !ok {
		return fmt.Errorf("node %+v not exist in the p2p connection", dest)
	}
	// if there's no client, create one and reuse it.
	if _, ok := p.clientPool[dest]; !ok {
		conn, err := grpc.NewClient(p.info2Host[dest], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("grpc client fail to connect: %w", err)
		}

		p.clientPool[dest] = &clientConnection{conn, rpcserver.NewReplicaConnClient(conn)}
	}

	_, err := p.clientPool[dest].client.HandleMessage(ctx, &rpcserver.HandleMessageRequest{
		Msg:  msg,
		From: &rpcserver.NodePosition{ShardID: p.me.ShardID, NodeID: p.me.NodeID},
		To:   &rpcserver.NodePosition{ShardID: dest.ShardID, NodeID: dest.NodeID},
	})
	if err != nil {
		return fmt.Errorf("grpc client fail to send message: %w", err)
	}

	return nil
}

func (p *P2PConn) add2LocalBuffer(msg *rpcserver.WrappedMsg) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	select {
	case p.msgBuffer <- msg:
	default:
		return fmt.Errorf("message buffer is full or closed")
	}

	return nil
}
