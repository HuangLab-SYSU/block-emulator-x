package pbft

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/insideop"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/outsideop"
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
	executeInterval = 500 * time.Millisecond
)

// Node is the node running the PBFT consensus.
type Node struct {
	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.
	pbftMeta *consensusMeta      // pbftMeta is the current consensus procedure

	iop insideop.ShardInsideOp
	omh outsideop.ShardOutsideMsgHandler

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
		if err := n.step2NextStage(ctx); err != nil {
			slog.ErrorContext(ctx, "step2NextStage", "err", err)
		}

		// if this node is the leader and not proposed yet, try to propose one block
		if n.pbftMeta.stage == stagePreprepare && n.pbftMeta.info.NodeID == n.pbftMeta.leader && !n.pbftMeta.proposed {
			if err := n.propose(ctx); err != nil {
				slog.ErrorContext(ctx, "propose", "err", err)
				continue
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
}

func (n *Node) handleMessage(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	handleFunc, exist := n.msgHandler[msg.GetMsgType()]
	if !exist {
		if err := n.omh.HandleMsgOutsideShard(ctx, msg); err != nil {
			return fmt.Errorf("handleMsgOutsideShard: %w", err)
		}

		return nil
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
		slog.InfoContext(ctx, "handle out-of-date Preprepare, ignore it", "view", ppMsg.View, "seq", ppMsg.Seq)
		return nil
	}

	if err := n.iop.ValidateProposal(ctx, &ppMsg.P); err != nil {
		return fmt.Errorf("validate preprepare proposal err: %w", err)
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
		slog.InfoContext(ctx, "handle out-of-date Prepare, ignore it", "view", pMsg.View, "seq", pMsg.Seq)
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
		slog.InfoContext(ctx, "handle out-of-date Commit, ignore it", "view", cMsg.View, "seq", cMsg.Seq)
		return nil
	}

	n.pbftMeta.msgPool.PushCommitMsg(&cMsg)

	return nil
}

// step2NextStage steps to next pbft stage until it steps to the end
func (n *Node) step2NextStage(ctx context.Context) error {
	for {
		oldStage, newStage, err := n.pbftMeta.step2Next()
		if err != nil {
			return fmt.Errorf("step2NextStage: %w", err)
		}
		// stages is unchanged, return
		if oldStage == newStage {
			return nil
		}

		switch newStage {
		case stagePreprepare:
			// deliver according to the last proposal
			if err = n.iop.ProposalCommitAndDeliver(ctx, &n.pbftMeta.lastProposal.P); err != nil {
				slog.ErrorContext(ctx, "deliver the last confirmed proposal failed", "err", err)
			}

			return nil
		case stagePrepare:
			// broadcast prepare message here
			if err = n.prepareBroadcast(ctx); err != nil {
				return fmt.Errorf("prepareBroadcast failed, err: %w", err)
			}
			// not return, go to next recursion
		case stageCommit:
			if err = n.commitBroadcast(ctx); err != nil {
				return fmt.Errorf("commitBroadcast failed, err: %w", err)
			}

			// not return, go to next recursion
		default:
			return fmt.Errorf("unknown stage: %d", newStage)
		}
	}
}

// propose build a proposal for this round and broadcast it to all followers.
func (n *Node) propose(ctx context.Context) error {
	p, err := n.iop.BuildProposal(ctx)
	if err != nil {
		return fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	// wrap and encode msg
	digest, err := utils.CalcHash(p)
	if err != nil {
		return fmt.Errorf("CalcHash failed: %w", err)
	}

	wrappedMsg, err := message.WrapMsg(&message.PreprepareMsg{
		P: *p, Digest: digest, Seq: n.pbftMeta.seq, View: n.pbftMeta.view,
	})
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.info.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.conn.GroupBroadcastMessage(ctx, shardNeighbors, wrappedMsg)

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

	n.conn.GroupBroadcastMessage(ctx, shardNeighbors, w)

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

	n.conn.GroupBroadcastMessage(ctx, shardNeighbors, w)

	return nil
}

func (n *Node) closeAll() {
	n.conn.Close()
	n.iop.Close()
	n.omh.Close()
}
