package pbft

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

type messageHandleFunc func(context.Context, []byte) error

const (
	initialLeader   = 0
	executeInterval = 1000 * time.Millisecond

	stagePreprepare = 0
	stagePrepare    = 1
	stageCommit     = 2
	stageNewView    = 3
	stageViewChange = 4
)

// consensusMeta is the metadata of PBFT consensus.
// Note that, consensusMeta is not thread-safe.
type consensusMeta struct {
	myAddr   [20]byte          // address in this blockchain
	info     nodetopo.NodeInfo // information of current block
	seq      int64             // sequence id of PBFT consensus.
	view     int64             // view of PBFT consensus.
	stage    int64             // stage of PBFT consensus, containing preprepare, prepare, commit, view-change and new-view.
	f        int64             // the number of fault-tolerance nodes
	leader   int64             // the leader id
	closed   bool              // consensus is closed or not
	proposed bool              // this node has proposed in this round or not

	cfg config.ConsensusCfg
}

// Node is the node running the PBFT consensus.
type Node struct {
	conn      network.P2PConn     // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver  nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.
	chain     *chain.Chain        // chain is the data-structure of blockchain.
	txPool    txpool.TxPool       // txPool is the transactions pool.
	msgPool   *message.MsgPool    // msgPool is the pool of messages, containing all ordered messages.
	logger    slog.Logger         // logger logs the information.
	localProc *consensusMeta      // localProc is the current consensus procedure

	msgHandler map[string]messageHandleFunc
}

// NewPBFTNode creates a new node running PBFT consensus with given configurations.
func NewPBFTNode(conn network.P2PConn, r nodetopo.NodeMapper, c *chain.Chain, txp txpool.TxPool, cfg config.ConsensusCfg) (*Node, error) {
	if cfg.ShardNum <= 0 || cfg.ShardNum <= conn.GetMyInfo().ShardID || cfg.NodeNum <= 0 || cfg.NodeNum <= conn.GetMyInfo().NodeID {
		return nil, fmt.Errorf("invalid configuration, shardID=%d, nodeID=%d, cfg=%+v", conn.GetMyInfo().ShardID, conn.GetMyInfo().NodeID, cfg)
	}

	if cfg.HandlerBufferSize <= 0 {
		return nil, fmt.Errorf("expected HandlerBufferSize > 0, got handlerBufferSize=%d", cfg.HandlerBufferSize)
	}

	return &Node{
		conn:     conn,
		resolver: r,
		chain:    c,
		msgPool:  message.NewMsgPool(),
		txPool:   txp,
		localProc: &consensusMeta{
			info:     conn.GetMyInfo(),
			seq:      0,
			view:     0,
			stage:    stagePreprepare,
			f:        (cfg.NodeNum - 1) / 3,
			leader:   initialLeader,
			closed:   false,
			proposed: false,
			cfg:      cfg,
		},
	}, nil
}

// Start starts the backend consensus logic.
// Note that it should be started with the another goroutine.
func (n *Node) Start() {
	// register message handle functions
	n.registerHandleFunc()

	// start and run
	n.run()
}

func (n *Node) run() {
	runTicker := time.NewTicker(executeInterval)
	defer runTicker.Stop()

	for range runTicker.C {
		// if the node is closed, break it
		if n.localProc.closed {
			break
		}

		// fetch messages from buffer
		n.msgPool.Put(n.conn.ReadMsgBuffer())

		// read messages from the pool, with the expected order, sequentially
		ms := n.msgPool.ReadTop(n.localProc.view, n.localProc.seq)
		for _, m := range ms {
			// handle messages in order one by one
			ctx := context.Background()
			err := n.handleMessage(ctx, m)
			if err != nil {
				n.logger.ErrorContext(ctx, "handleMessage", "msg", m, "err", err)
			}
		}

		// if this node is the leader and not proposed yet, try to propose one block
		if n.localProc.info.NodeID == n.localProc.leader && !n.localProc.proposed {
			ctx := context.Background()
			err := n.propose(ctx)
			if err != nil {
				n.logger.ErrorContext(ctx, "propose", "err", err)
			}

			n.localProc.proposed = true
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

	err := handleFunc(ctx, msg.GetPayload())
	if err != nil {
		return fmt.Errorf("handle %s message err: %w", msg.GetMsgType(), err)
	}

	return nil
}

func (n *Node) handlePreprepare(ctx context.Context, payload []byte) error {
	panic("implement me")
}

func (n *Node) handlePrepare(ctx context.Context, payload []byte) error {
	panic("implement me")
}

func (n *Node) handleCommit(ctx context.Context, payload []byte) error {
	panic("implement me")
}

func (n *Node) handleReceiveTxs(ctx context.Context, payload []byte) error {
	panic("implement me")
}

func (n *Node) propose(ctx context.Context) error {
	txs, err := n.txPool.PackTxs()
	if err != nil {
		return fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	b, err := n.chain.GenerateBlock(ctx, n.localProc.myAddr, txs)
	if err != nil {
		return fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	n.logger.InfoContext(ctx, "block generated", "block height", b.Header.Number, "block create time", b.Header.CreateTime)

	// wrap and encode msg
	digest, err := utils.CalcHash(b)
	if err != nil {
		return fmt.Errorf("CalcHash failed: %w", err)
	}

	wrappedMsg, err := message.WrapMsg(&message.PreprepareMsg{
		B: *b, Digest: digest, Seq: n.localProc.seq, View: n.localProc.view,
	})
	if err != nil {
		return fmt.Errorf("message.WrapMsg failed: %w", err)
	}

	shardNeighbors, err := n.resolver.GetNodesInShard(n.localProc.info.ShardID)
	if err != nil {
		return fmt.Errorf("GetNodesInShard failed: %w", err)
	}

	n.broadcast2Nodes(ctx, shardNeighbors, wrappedMsg)

	return nil
}

func (n *Node) broadcast2Nodes(ctx context.Context, dest []nodetopo.NodeInfo, msg *rpcserver.WrappedMsg) {
	// broadcast to all nodes in this shard.
	for _, neighbor := range dest {
		go func(nb nodetopo.NodeInfo) {
			err := n.conn.SendMessage(ctx, nb, msg)
			if err != nil {
				n.logger.ErrorContext(ctx, "sub-goroutine: broadcast", "err", err)
			}
		}(neighbor)
	}
}

func (n *Node) closeAll() {
	n.conn.Close()
	_ = n.chain.Close()
}
