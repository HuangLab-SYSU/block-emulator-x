package pbft

import (
	"bytes"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/message"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/pool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

const (
	initialLeader = 0

	stagePreprepare = 0
	stagePrepare    = 1
	stageCommit     = 2
	stageNewView    = 3
	stageViewChange = 4
)

// consensusMeta is the metadata of PBFT consensus.
// Note that, consensusMeta is not thread-safe.
type consensusMeta struct {
	cfg config.ConsensusCfg

	addr   account.Address   // address in this blockchain
	info   nodetopo.NodeInfo // information of current node
	f      int64             // the number of fault-tolerance nodes
	leader int64             // the leader id
	closed bool              // consensus is closed or not

	msgPool *pool.MsgPool

	// metadata for a single round;
	// variables below will update per round.
	proposed    bool                           // this node has proposed in this round or not
	seq         int64                          // sequence id of PBFT consensus.
	view        int64                          // view of PBFT consensus.
	stage       int64                          // stage of PBFT consensus, containing preprepare, prepare, commit, view-change and new-view.
	curProposal *message.PreprepareMsg         // preprepare message in this round.
	prepareSet  map[nodetopo.NodeInfo]struct{} // prepareSet collects the nodes sending prepare message.
	commitSet   map[nodetopo.NodeInfo]struct{} // commitSet collects the nodes sending commit message.
}

func newConsensusMeta(cfg config.ConsensusCfg) *consensusMeta {
	return &consensusMeta{
		cfg: cfg,

		info:   nodetopo.NodeInfo{NodeID: cfg.NodeID, ShardID: cfg.ShardID},
		addr:   cfg.WalletAddr,
		f:      (cfg.NodeNum - 1) / 3,
		leader: initialLeader,
		closed: false,

		msgPool: pool.NewMsgPool(),

		proposed:    false,
		seq:         0,
		view:        0,
		stage:       stagePreprepare,
		curProposal: nil,
		prepareSet:  map[nodetopo.NodeInfo]struct{}{},
		commitSet:   map[nodetopo.NodeInfo]struct{}{},
	}
}

// curateMsg counts the number of valid messages in the msgPool.
func (c *consensusMeta) curateMsg() {
	if ppMsg := c.msgPool.ReadPreprepareMsg(c.view, c.seq); len(ppMsg) > 0 {
		c.curProposal = ppMsg[0]
	}

	pMsgList := c.msgPool.ReadPrepareMsg(c.stage, c.seq)
	for _, pMsg := range pMsgList {
		if !bytes.Equal(pMsg.Digest, c.curProposal.Digest) {
			// if the message is invalid (wrong digest), drop it
			continue
		}

		c.prepareSet[nodetopo.NodeInfo{NodeID: pMsg.NodeID, ShardID: pMsg.ShardID}] = struct{}{}
	}

	cMsgList := c.msgPool.ReadCommitMsg(c.stage, c.seq)
	for _, cMsg := range cMsgList {
		if !bytes.Equal(cMsg.Digest, c.curProposal.Digest) {
			// if the message is invalid (wrong digest), drop it
			continue
		}

		c.commitSet[nodetopo.NodeInfo{NodeID: cMsg.NodeID, ShardID: cMsg.ShardID}] = struct{}{}
	}
}

func (c *consensusMeta) step2Next() (int, error) {
	switch c.stage {
	case stagePreprepare:
		if c.curProposal == nil {
			return -1, fmt.Errorf("preprepare is nil")
		}

		c.stage = stagePrepare

		return stagePrepare, nil

	case stagePrepare:
		if len(c.prepareSet) < int(2*c.f+1) {
			return -1, fmt.Errorf("prepareSet is %d < %d", len(c.prepareSet), int(2*c.f+1))
		}

		c.stage = stageCommit

		return stageCommit, nil

	case stageCommit:
		if len(c.commitSet) < int(2*c.f+1) {
			return -1, fmt.Errorf("commitSet is %d < %d", len(c.commitSet), int(2*c.f+1))
		}

		// step into next round
		c.stage = stagePreprepare
		c.curProposal = nil
		c.prepareSet = map[nodetopo.NodeInfo]struct{}{}
		c.commitSet = map[nodetopo.NodeInfo]struct{}{}
		c.proposed = false
		c.seq++

		return stagePreprepare, nil

	default:

	}

	return -1, fmt.Errorf("invalid stage %d", c.stage)
}
