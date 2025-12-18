package pbft

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/basicstructs"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/insideop"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/outsideop"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/csvwrite"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

type messageHandleFunc func(context.Context, []byte) error

const (
	// executeInterval is the time interval between two running loops.
	executeInterval    = 500 * time.Millisecond
	blockRecordPathFmt = "shard=%d_node=%d/block_record.csv"
)

// Node is the node running the PBFT consensus.
type Node struct {
	conn     *network.ConnHandler // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper  // resolver gives the information of all consensus nodes and shards.
	pbftMeta *consensusMeta       // pbftMeta is the current consensus procedure

	bc  *chain.Chain
	csw *csvwrite.CSVSeqWriter

	iop insideop.ShardInsideOp
	omh outsideop.ShardOutsideMsgHandler

	pbftMsgHandler map[string]messageHandleFunc
}

// NewPBFTNode creates a new node running PBFT consensus with given configurations.
func NewPBFTNode(conn *network.ConnHandler, r nodetopo.NodeMapper, cfg config.ConsensusNodeCfg, lp config.LocalParams) (n *Node, rErr error) {
	if cfg.ShardNum <= 0 || cfg.ShardNum <= lp.ShardID {
		return nil, fmt.Errorf("invalid shardID=%d", lp.ShardID)
	}

	if cfg.NodeNum <= 0 || cfg.NodeNum <= lp.NodeID {
		return nil, fmt.Errorf("invalid nodeID=%d", lp.NodeID)
	}

	var cleanups []func()

	defer func() {
		if rErr != nil {
			for i := len(cleanups) - 1; i >= 0; i-- {
				cleanups[i]()
			}
		}
	}()

	// New a blockchain.
	bc, err := chain.NewChain(cfg.BlockchainCfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewChain err=%w", err)
	}

	cleanups = append(cleanups, func() { _ = bc.Close() })

	// New a transaction pool.
	txp, err := txpool.NewTxPool(cfg.TxPoolCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxPool err=%w", err)
	}

	// New a transaction block operation.
	tbo, err := txblockop.NewTxBlockOp(conn, r, bc, cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("NewTxBlockOp failed: %w", err)
	}

	// New a csv writer to record blocks.
	csw, err := csvwrite.NewCSVSeqWriter(filepath.Join(cfg.BlockRecordDir, fmt.Sprintf(blockRecordPathFmt, lp.ShardID, lp.NodeID)), block.RecordTitle)
	if err != nil {
		return nil, fmt.Errorf("NewCSVSeqWriter failed: %w", err)
	}

	cleanups = append(cleanups, func() { _ = csw.Close() })

	var (
		iop insideop.ShardInsideOp
		omh outsideop.ShardOutsideMsgHandler
	)

	switch cfg.ConsensusType {
	case config.StaticRelayConsensus, config.StaticBrokerConsensus:
		omh = outsideop.NewStaticLocOutsideOp(txp)
		iop = insideop.NewStaticShardOp(bc, txp, tbo, csw, cfg)
	case config.CLPARelayConsensus, config.CLPABrokerConsensus:
		amm := migration.NewAccMigrateMetadata(cfg.SystemCfg, lp)
		omh = outsideop.NewCLPALocOutsideOp(txp, amm)
		iop = insideop.NewDynamicShardOp(conn, r, bc, txp, amm, tbo, csw, cfg, lp)
	default:
		return nil, fmt.Errorf("invalid consensus type=%s", cfg.ConsensusType)
	}

	return &Node{
		conn:     conn,
		resolver: r,
		pbftMeta: newConsensusMeta(cfg, lp),

		bc: bc,

		iop: iop,
		omh: omh,

		pbftMsgHandler: make(map[string]messageHandleFunc),
	}, nil
}

// Start starts the backend consensus logic.
// Note that it should be started with the another goroutine.
func (n *Node) Start() {
	n.registerHandleFunc()

	// Start and run.
	n.run()
}

func (n *Node) run() {
	runTicker := time.NewTicker(executeInterval)
	defer runTicker.Stop()

	for range runTicker.C {
		ctx := context.Background()
		// If the node is closed, break it.
		if n.pbftMeta.closed {
			break
		}

		// Fetch messages from buffer to pool.
		msgList := n.conn.DrainMsgBuffer()
		for _, msg := range msgList {
			err := n.handleMessage(ctx, msg)
			if err != nil {
				slog.ErrorContext(ctx, "handleMessage failed", "err", err)
			}
		}

		// Update PBFT process.
		n.pbftMeta.curateMsg()

		// Try to step into the next process.
		if err := n.step2NextStage(ctx); err != nil {
			slog.ErrorContext(ctx, "step2NextStage failed", "err", err)
		}

		if n.pbftMeta.lp.NodeID == n.pbftMeta.leader {
			// If this node is the leader and not proposed yet, try to propose one block.
			if n.pbftMeta.stage == stagePreprepare && !n.pbftMeta.proposed {
				if err := n.propose(ctx); err != nil {
					slog.ErrorContext(ctx, "propose failed", "err", err)
				}
			}
		} else if n.pbftMeta.catchupReady() {
			// If this node is not the leader, check whether to use catch-up.
			if err := n.catchUpStart(ctx); err != nil {
				slog.ErrorContext(ctx, "catchupStart failed", "err", err)
			}
		}
	}

	n.closeAll()
}

// registerHandleFunc registers all message handle functions.
func (n *Node) registerHandleFunc() {
	n.pbftMsgHandler[message.StopConsensusMessageType] = n.handleStopConsensus
	n.pbftMsgHandler[message.PreprepareMessageType] = n.handlePreprepare
	n.pbftMsgHandler[message.PrepareMessageType] = n.handlePrepare
	n.pbftMsgHandler[message.CommitMessageType] = n.handleCommit
	n.pbftMsgHandler[message.CatchupReqMessageType] = n.handleCatchupReq
	n.pbftMsgHandler[message.CatchupRespMessageType] = n.handleCatchupResp
}

func (n *Node) handleMessage(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	handleFunc, exist := n.pbftMsgHandler[msg.GetMsgType()]
	if !exist {
		if err := n.omh.HandleMsgOutsideShard(ctx, msg); err != nil {
			return fmt.Errorf("handleMsgOutsideShard failed: %w", err)
		}

		return nil
	}

	if err := handleFunc(ctx, msg.GetPayload()); err != nil {
		return fmt.Errorf("handle %s message failed: %w", msg.GetMsgType(), err)
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

	slog.InfoContext(ctx, "handle preprepare message: try to add it to the message pool", "shardID", ppMsg.ShardID, "nodeID", ppMsg.NodeID)

	// ignore the out-of-date message
	if n.pbftMeta.curViewSeq.Compare(basicstructs.ViewSeq{View: ppMsg.View, Seq: ppMsg.Seq}) > 0 {
		slog.InfoContext(ctx, "handle out-of-date Preprepare, ignore it", "view", ppMsg.View, "seq", ppMsg.Seq)
		return nil
	}

	if err := n.iop.ValidateProposal(ctx, &ppMsg.P); err != nil {
		return fmt.Errorf("validate preprepare proposal err: %w", err)
	}

	n.pbftMeta.msgPool.PushPreprepareMsg(&ppMsg)
	n.pbftMeta.updateLatestViewSeq(ppMsg.View, ppMsg.Seq)

	return nil
}

func (n *Node) handlePrepare(ctx context.Context, payload []byte) error {
	var pMsg message.PrepareMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&pMsg); err != nil {
		return fmt.Errorf("decode prepare msg: %w", err)
	}

	slog.InfoContext(ctx, "handle prepare message: try to add it to the message pool", "shardID", pMsg.ShardID, "nodeID", pMsg.NodeID)

	// ignore the out-of-date message
	if n.pbftMeta.curViewSeq.Compare(basicstructs.ViewSeq{View: pMsg.View, Seq: pMsg.Seq}) > 0 {
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

	slog.InfoContext(ctx, "handle commit message: try to add it to the message pool", "shardID", cMsg.ShardID, "nodeID", cMsg.NodeID)

	// ignore the out-of-date message
	if n.pbftMeta.curViewSeq.Compare(basicstructs.ViewSeq{View: cMsg.View, Seq: cMsg.Seq}) > 0 {
		slog.InfoContext(ctx, "handle out-of-date Commit, ignore it", "view", cMsg.View, "seq", cMsg.Seq)
		return nil
	}

	n.pbftMeta.msgPool.PushCommitMsg(&cMsg)

	return nil
}

func (n *Node) handleCatchupReq(ctx context.Context, payload []byte) error {
	var crMsg message.CatchupReqMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&crMsg); err != nil {
		return fmt.Errorf("decode catchup req msg: %w", err)
	}

	slog.InfoContext(ctx, "handle catch up req message", "from shardID", crMsg.ShardID, "from nodeID", crMsg.NodeID)

	blocks, err := n.bc.GetBlocksAfterHeight(crMsg.ShardID)
	if err != nil {
		return fmt.Errorf("get blocks failed: %w", err)
	}

	proposals := make([]message.Proposal, len(blocks))
	for i, b := range blocks {
		proposals[i] = *message.WrapProposal(&b)
	}

	w, err := message.WrapMsg(&message.CatchupRespMsg{
		Proposals: proposals,
		NextView:  n.pbftMeta.curViewSeq.View,
		NextSeq:   n.pbftMeta.curViewSeq.Seq,
		ShardID:   n.pbftMeta.lp.ShardID,
		NodeID:    n.pbftMeta.lp.NodeID,
	})
	if err != nil {
		return fmt.Errorf("wrap message failed: %w", err)
	}

	n.conn.SendMsg2Dest(ctx, nodetopo.NodeInfo{NodeID: crMsg.NodeID, ShardID: crMsg.ShardID}, w)

	return nil
}

func (n *Node) handleCatchupResp(ctx context.Context, payload []byte) error {
	var crMsg message.CatchupRespMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&crMsg); err != nil {
		return fmt.Errorf("decode catchup resp msg: %w", err)
	}

	slog.InfoContext(ctx, "handle catch up req message")

	if n.pbftMeta.leader != n.pbftMeta.lp.NodeID {
		return fmt.Errorf("the leader will not catch up")
	}

	if !n.pbftMeta.catchupStarted {
		return fmt.Errorf("catchup req message not started")
	}

	// Add blocks.
	for _, p := range crMsg.Proposals {
		if err := n.bc.AddBlock(ctx, p.Block); err != nil {
			return fmt.Errorf("add block failed, height=%d, err: %w", p.Block.Number, err)
		}
	}

	// Set catchup states.
	n.pbftMeta.catchupOverAndReset(basicstructs.ViewSeq{View: crMsg.NextView, Seq: crMsg.NextSeq})

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
		slog.Debug("not the time to propose, ignore it", "time duration", time.Since(n.pbftMeta.lastProposeTime).Seconds())
		return nil
	}

	slog.Debug("try to propose")

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

	ppMsg := &message.PreprepareMsg{
		P: *p, Digest: digest, Seq: n.pbftMeta.curViewSeq.Seq, View: n.pbftMeta.curViewSeq.View,
	}

	if err = n.broadcastMsgInnerShard(ctx, ppMsg); err != nil {
		return fmt.Errorf("broadcastMsgInnerShard failed: %w", err)
	}

	n.pbftMeta.proposed = true
	n.pbftMeta.lastProposeTime = time.Now()

	return nil
}

func (n *Node) prepareBroadcast(ctx context.Context) error {
	pMsg := &message.PrepareMsg{
		Digest:  n.pbftMeta.curProposal.Digest,
		View:    n.pbftMeta.curProposal.View,
		Seq:     n.pbftMeta.curProposal.Seq,
		ShardID: n.pbftMeta.lp.ShardID,
		NodeID:  n.pbftMeta.lp.NodeID,
	}

	return n.broadcastMsgInnerShard(ctx, pMsg)
}

func (n *Node) commitBroadcast(ctx context.Context) error {
	cMsg := &message.CommitMsg{
		Digest:  n.pbftMeta.curProposal.Digest,
		View:    n.pbftMeta.curProposal.View,
		Seq:     n.pbftMeta.curProposal.Seq,
		ShardID: n.pbftMeta.lp.ShardID,
		NodeID:  n.pbftMeta.lp.NodeID,
	}

	return n.broadcastMsgInnerShard(ctx, cMsg)
}

func (n *Node) catchUpStart(ctx context.Context) error {
	// Send the catchup message to the leader.
	leader, err := n.resolver.GetLeader(n.pbftMeta.lp.ShardID)
	if err != nil {
		return fmt.Errorf("get leader failed: %w", err)
	}

	w, err := message.WrapMsg(&message.CatchupReqMsg{
		StartBlockHeight: int64(n.bc.GetCurHeader().Number) + 1, // StartBlockHeight should be the current height + 1
		ShardID:          n.pbftMeta.lp.ShardID,
		NodeID:           n.pbftMeta.lp.NodeID,
	})
	if err != nil {
		return fmt.Errorf("wrap message failed: %w", err)
	}

	n.conn.SendMsg2Dest(ctx, leader, w)
	n.pbftMeta.catchupStarted = true

	return nil
}

func (n *Node) broadcastMsgInnerShard(ctx context.Context, msg any) error {
	w, err := message.WrapMsg(msg)
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
	slog.Info("consensus node is closing")

	n.conn.Close()
	_ = n.bc.Close()
	_ = n.csw.Close()
}
