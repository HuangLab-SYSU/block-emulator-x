package network

import (
	"context"
	"sync"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type P2PConn interface {
	StartServer() error
	GetMeNodeInfo() nodetopo.NodeInfo
	ReadMsgBuffer() []*rpcserver.WrappedMsg
	SendMessage(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg)
	Close()
}

type ConnHandler struct {
	P2PConn
}

func NewConnHandler(p2p P2PConn) *ConnHandler {
	return &ConnHandler{P2PConn: p2p}
}

func (p *ConnHandler) MSendDifferentMessages(ctx context.Context, node2Msg map[nodetopo.NodeInfo]*rpcserver.WrappedMsg) {
	wg := sync.WaitGroup{}
	wg.Add(len(node2Msg))

	for node, msg := range node2Msg {
		go func(node nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
			defer wg.Done()

			p.SendMessage(ctx, node, msg)
		}(node, msg)
	}

	wg.Wait()
}

func (p *ConnHandler) GroupBroadcastMessage(ctx context.Context, group []nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	wg := &sync.WaitGroup{}
	wg.Add(len(group))
	// broadcast to all nodes in this group
	for _, node := range group {
		go func(nif nodetopo.NodeInfo) {
			defer wg.Done()

			p.SendMessage(ctx, nif, msg)
		}(node)
	}

	wg.Wait()
}
