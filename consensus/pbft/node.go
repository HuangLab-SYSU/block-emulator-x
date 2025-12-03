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
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/outsideop"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
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

	defaultMsgHandler map[string]messageHandleFunc
}

// NewPBFTNode creates a new node running PBFT consensus with given configurations.
func NewPBFTNode(conn *network.P2PConn, r nodetopo.NodeMapper, cfg config.ConsensusNodeCfg, lp config.LocalParams) (*Node, error) {
	if cfg.ShardNum <= 0 || cfg.ShardNum <= lp.ShardID {
		return nil, fmt.Errorf("invalid shardID=%d", lp.ShardID)
	}

	if cfg.NodeNum <= 0 || cfg.NodeNum <= lp.NodeID {
		return nil, fmt.Errorf("invalid nodeID=%d", lp.NodeID)
	}

	// new a blockchain
	bc, err := chain.NewChain(cfg.BlockchainCfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewChain err=%w", err)
	}

	txp, err := txpool.NewTxPool(cfg.TxPoolCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxPool err=%w", err)
	}

	var (
		iop insideop.ShardInsideOp
		omh outsideop.ShardOutsideMsgHandler
	)

	switch cfg.ConsensusType {
	case config.StaticRelayConsensus:
		omh = outsideop.NewStaticLocOutsideOp(txp)
		if iop, err = insideop.NewStaticRelayInsideOp(conn, r, bc, txp, cfg, lp); err != nil {
			return nil, fmt.Errorf("NewStaticRelayInsideOp err=%w", err)
		}

	case config.StaticBrokerConsensus:
		omh = outsideop.NewStaticLocOutsideOp(txp)
		if iop, err = insideop.NewStaticBrokerInsideOp(conn, r, bc, txp, cfg, lp); err != nil {
			return nil, fmt.Errorf("NewStaticBrokerInsideOp err=%w", err)
		}

	case config.CLPARelayConsensus:
		amm := migration.NewAccMigrateMetadata(cfg.SystemCfg, lp)

		omh = outsideop.NewCLPALocOutsideOp(txp, amm)
		if iop, err = insideop.NewCLPARelayInsideOp(conn, r, bc, txp, amm, cfg, lp); err != nil {
			return nil, fmt.Errorf("NewCLPARelayInsideOp err=%w", err)
		}

	case config.CLPABrokerConsensus:
		amm := migration.NewAccMigrateMetadata(cfg.SystemCfg, lp)

		omh = outsideop.NewCLPALocOutsideOp(txp, amm)
		if iop, err = insideop.NewCLPABrokerInsideOp(conn, r, bc, txp, amm, cfg, lp); err != nil {
			return nil, fmt.Errorf("NewCLPABrokerInsideOp err=%w", err)
		}

	default:
		return nil, fmt.Errorf("invalid consensus type=%s", cfg.ConsensusType)
	}

	return &Node{
		conn:     conn,
		resolver: r,
		pbftMeta: newConsensusMeta(cfg, lp),

		iop: iop,
		omh: omh,

		defaultMsgHandler: make(map[string]messageHandleFunc),
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
		if n.pbftMeta.stage == stagePreprepare && n.pbftMeta.lp.NodeID == n.pbftMeta.leader && !n.pbftMeta.proposed {
			if err := n.propose(ctx); err != nil {
				slog.ErrorContext(ctx, "propose", "err", err)
			}
		}
	}

	n.closeAll()
}

// registerHandleFunc registers all message handle functions.
func (n *Node) registerHandleFunc() {
	n.defaultMsgHandler[message.StopConsensusMessageType] = n.handleStopConsensus
	n.defaultMsgHandler[message.PreprepareMessageType] = n.handlePreprepare
	n.defaultMsgHandler[message.PrepareMessageType] = n.handlePrepare
	n.defaultMsgHandler[message.CommitMessageType] = n.handleCommit
}

func (n *Node) handleMessage(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	handleFunc, exist := n.defaultMsgHandler[msg.GetMsgType()]
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

func (n *Node) handleStopConsensus(ctx context.Context, _ []byte) error {
	n.pbftMeta.closed = true

	slog.InfoContext(ctx, "handle stop consensus message")

	return nil
}

func (n *Node) handlePreprepare(ctx context.Context, payload []byte) error {
	var ppMsg message.PreprepareMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&ppMsg); err != nil {
		return fmt.Errorf("decode preprepare msg: %w", err)
	}

	slog.InfoContext(ctx, "handle preprepare message", "shardID", ppMsg.ShardID, "nodeID", ppMsg.NodeID)

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

	slog.InfoContext(ctx, "handle prepare message", "shardID", pMsg.ShardID, "nodeID", pMsg.NodeID)

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

	slog.InfoContext(ctx, "handle commit message", "shardID", cMsg.ShardID, "nodeID", cMsg.NodeID)

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
			if err = n.iop.ProposalCommitAndDeliver(ctx, n.pbftMeta.leader == n.pbftMeta.lp.NodeID, &n.pbftMeta.lastProposal.P); err != nil {
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
	if time.Since(n.pbftMeta.lastProposeTime) < time.Duration(n.pbftMeta.cfg.BlockInterval)*time.Millisecond {
		// not reach the block interval
		return nil
	}

	p, err := n.iop.BuildProposal(ctx)
	if err != nil {
		return fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	// If the proposal is nil, skip this propose operation.
	if p == nil {
		return nil
	}

	// wrap and encode msg
	digest, err := p.Hash()
	if err != nil {
		return fmt.Errorf("CalcHash failed: %w", err)
	}

	wrappedMsg, err := message.WrapMsg(&message.PreprepareMsg{
		P: *p, Digest: digest, Seq: n.pbftMeta.seq, View: n.pbftMeta.view,
	})
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.lp.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.conn.GroupBroadcastMessage(ctx, shardNeighbors, wrappedMsg)
	n.pbftMeta.proposed = true
	n.pbftMeta.lastProposeTime = time.Now()

	return nil
}

func (n *Node) prepareBroadcast(ctx context.Context) error {
	pMsg := &message.PrepareMsg{
		Digest:  n.pbftMeta.curProposal.Digest,
		View:    n.pbftMeta.view,
		Seq:     n.pbftMeta.seq,
		ShardID: n.pbftMeta.lp.ShardID,
		NodeID:  n.pbftMeta.lp.NodeID,
	}

	w, err := message.WrapMsg(pMsg)
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.lp.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.conn.GroupBroadcastMessage(ctx, shardNeighbors, w)

	return nil
}

func (n *Node) commitBroadcast(ctx context.Context) error {
	cMsg := &message.CommitMsg{
		Digest:  n.pbftMeta.curProposal.Digest,
		View:    n.pbftMeta.view,
		Seq:     n.pbftMeta.seq,
		ShardID: n.pbftMeta.lp.ShardID,
		NodeID:  n.pbftMeta.lp.NodeID,
	}

	w, err := message.WrapMsg(cMsg)
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.pbftMeta.lp.ShardID)
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
