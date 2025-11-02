package pbft

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

type messageHandleFunc func(context.Context, []byte) error

const (
	executeInterval = 1000 * time.Millisecond
)

// Node is the node running the PBFT consensus.
type Node struct {
	conn       *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver   nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.
	chain      *chain.Chain        // chain is the data-structure of blockchain.
	txPool     txpool.TxPool       // txPool is the transactions pool.
	pbftMeta   *consensusMeta      // pbftMeta is the current consensus procedure
	msgHandler map[string]messageHandleFunc
}

// NewPBFTNode creates a new node running PBFT consensus with given configurations.
func NewPBFTNode(conn *network.P2PConn, r nodetopo.NodeMapper, c *chain.Chain, txp txpool.TxPool, cfg config.ConsensusCfg) (*Node, error) {
	if cfg.ShardNum <= 0 || cfg.ShardNum <= cfg.ShardID {
		return nil, fmt.Errorf("invalid shardID=%d", cfg.ShardID)
	}

	if cfg.NodeNum <= 0 || cfg.NodeNum <= cfg.NodeID {
		return nil, fmt.Errorf("invalid nodeID=%d", cfg.NodeID)
	}

	if cfg.HandlerBufferSize <= 0 {
		return nil, fmt.Errorf("expected HandlerBufferSize > 0, got handlerBufferSize=%d", cfg.HandlerBufferSize)
	}

	return &Node{
		conn:     conn,
		resolver: r,
		chain:    c,
		txPool:   txp,
		pbftMeta: newConsensusMeta(cfg),
	}, nil
}

// Start starts the backend consensus logic.
// Note that it should be started with the another goroutine.
func (n *Node) Start() {
	n.registerHandleFunc()

	// start and run
	n.run()
}

func (n *Node) run() {
	runTicker := time.NewTicker(executeInterval)
	defer runTicker.Stop()

	for range runTicker.C {
		ctx := context.Background()
		// if the node is closed, break it
		if n.pbftMeta.closed {
			break
		}

		// fetch messages from buffer to pool
		msgList := n.conn.ReadMsgBuffer()
		for _, msg := range msgList {
			err := n.handleMessage(ctx, msg)
			if err != nil {
				slog.ErrorContext(ctx, "handleMessage", "err", err)
			}
		}

		// update PBFT process
		n.pbftMeta.curateMsg()

		// try to step into the next process
		for {
			if err := n.step2NextStage(ctx); err != nil {
				slog.ErrorContext(ctx, "step2NextStage", "err", err)
				break
			}
		}

		// if this node is the leader and not proposed yet, try to propose one block
		if n.pbftMeta.stage == stagePreprepare && n.pbftMeta.info.NodeID == n.pbftMeta.leader && !n.pbftMeta.proposed {
			err := n.propose(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "propose", "err", err)
			}

			n.pbftMeta.proposed = true
		}
	}

	n.closeAll()
}

// registerHandleFunc registers all message handle functions.
func (n *Node) registerHandleFunc() {
	n.msgHandler[message.PreprepareMessageType] = n.handlePreprepare
	n.msgHandler[message.PrepareMessageType] = n.handlePrepare
	n.msgHandler[message.CommitMessageType] = n.handleCommit
	n.msgHandler[message.ReceiveTxsMessageType] = n.handleReceiveTxs
}

func (n *Node) handleMessage(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	handleFunc, exist := n.msgHandler[msg.GetMsgType()]
	if !exist {
		return fmt.Errorf("invalid msg type: %s", msg.GetMsgType())
	}

	if err := handleFunc(ctx, msg.GetPayload()); err != nil {
		return fmt.Errorf("handle %s message err: %w", msg.GetMsgType(), err)
	}

	return nil
}

func (n *Node) handlePreprepare(ctx context.Context, payload []byte) error {
	var ppMsg message.PreprepareMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&ppMsg); err != nil {
		return fmt.Errorf("decode preprepare msg: %w", err)
	}

	// ignore the out-of-date message
	if ppMsg.View < n.pbftMeta.view || ppMsg.Seq < n.pbftMeta.seq {
		slog.InfoContext(ctx, "handle out-of-date Preprepare", "view", ppMsg.View, "seq", ppMsg.Seq)
		return nil
	}

	n.pbftMeta.msgPool.PushPreprepareMsg(&ppMsg)

	return nil
}

func (n *Node) handlePrepare(ctx context.Context, payload []byte) error {
	var pMsg message.PrepareMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&pMsg); err != nil {
		return fmt.Errorf("decode prepare msg: %w", err)
	}

	// ignore the out-of-date message
	if pMsg.View < n.pbftMeta.view || pMsg.Seq < n.pbftMeta.seq {
		slog.InfoContext(ctx, "handle out-of-date Prepare", "view", pMsg.View, "seq", pMsg.Seq)
		return nil
	}

	n.pbftMeta.msgPool.PushPrepareMsg(&pMsg)

	return nil
}

func (n *Node) handleCommit(ctx context.Context, payload []byte) error {
	var cMsg message.CommitMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&cMsg); err != nil {
		return fmt.Errorf("decode commit msg: %w", err)
	}

	// ignore the out-of-date message
	if cMsg.View < n.pbftMeta.view || cMsg.Seq < n.pbftMeta.seq {
		slog.InfoContext(ctx, "handle out-of-date Preprepare", "view", cMsg.View, "seq", cMsg.Seq)
		return nil
	}

	n.pbftMeta.msgPool.PushCommitMsg(&cMsg)

	return nil
}

func (n *Node) handleReceiveTxs(ctx context.Context, payload []byte) error {
	var txsMsg message.ReceiveTxsMsg

	err := gob.NewDecoder(bytes.NewBuffer(payload)).Decode(&txsMsg)
	if err != nil {
		return fmt.Errorf("decode receive txs msg: %w", err)
	}

	err = n.txPool.AddTxs(txsMsg.Txs)
	if err != nil {
		return fmt.Errorf("add txs: %w", err)
	}

	return nil
}

func (n *Node) step2NextStage(ctx context.Context) error {
	newStage, err := n.pbftMeta.step2Next()
	if err != nil {
		return fmt.Errorf("pbftMeta.step2Next failed: %w", err)
	}

	switch newStage {
	case stagePreprepare:
		// nothing to do in Preprepare stages
	case stagePrepare:
		// broadcast prepare message here
		if err = n.prepareBroadcast(ctx); err != nil {
			return fmt.Errorf("prepareBroadcast: %w", err)
		}
	case stageCommit:
		if err = n.commitBroadcast(ctx); err != nil {
			return fmt.Errorf("commitBroadcast: %w", err)
		}
	}

	return nil
}

func (n *Node) propose(ctx context.Context) error {
	txs, err := n.txPool.PackTxs()
	if err != nil {
		return fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	b, err := n.chain.GenerateBlock(ctx, n.pbftMeta.addr, txs)
	if err != nil {
		return fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	slog.InfoContext(ctx, "block generated", "block height", b.Header.Number, "block create time", b.Header.CreateTime)

	// wrap and encode msg
	digest, err := utils.CalcHash(b)
	if err != nil {
		return fmt.Errorf("CalcHash failed: %w", err)
	}

	wrappedMsg, err := message.WrapMsg(&message.PreprepareMsg{
		B: *b, Digest: digest, Seq: n.pbftMeta.seq, View: n.pbftMeta.view,
	})
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.info.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.broadcast2Nodes(ctx, shardNeighbors, wrappedMsg)

	return nil
}

func (n *Node) prepareBroadcast(ctx context.Context) error {
	pMsg := message.PrepareMsg{
		Digest:  n.pbftMeta.curProposal.Digest,
		View:    n.pbftMeta.view,
		Seq:     n.pbftMeta.seq,
		ShardID: n.pbftMeta.cfg.ShardID,
		NodeID:  n.pbftMeta.cfg.NodeID,
	}

	w, err := message.WrapMsg(pMsg)
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.info.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.broadcast2Nodes(ctx, shardNeighbors, w)

	return nil
}

func (n *Node) commitBroadcast(ctx context.Context) error {
	cMsg := message.CommitMsg{
		Digest:  n.pbftMeta.curProposal.Digest,
		View:    n.pbftMeta.view,
		Seq:     n.pbftMeta.seq,
		ShardID: n.pbftMeta.cfg.ShardID,
		NodeID:  n.pbftMeta.cfg.NodeID,
	}

	w, err := message.WrapMsg(cMsg)
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.info.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.broadcast2Nodes(ctx, shardNeighbors, w)

	return nil
}

func (n *Node) broadcast2Nodes(ctx context.Context, dest []nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	// broadcast to all nodes in this shard.
	for _, neighbor := range dest {
		go func(nb nodetopo.NodeInfo) {
			err := n.conn.SendMessage(ctx, nb, msg)
			if err != nil {
				slog.ErrorContext(ctx, "sub-goroutine: broadcast", "err", err)
			}
		}(neighbor)
	}
}

func (n *Node) closeAll() {
	n.conn.Close()
	_ = n.chain.Close()
}
