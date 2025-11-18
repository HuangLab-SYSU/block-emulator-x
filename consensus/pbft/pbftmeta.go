package pbft

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/pool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
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
	lp  config.LocalParams

	info   nodetopo.NodeInfo // information of current node
	f      int64             // the number of fault-tolerance nodes
	leader int64             // the leader id
	closed bool              // consensus is closed or not

	msgPool *pool.MsgPool

	// metadata for a single round;
	// variables below will update per round.
	proposed     bool                           // this node has proposed in this round or not
	seq          int64                          // sequence id of PBFT consensus.
	view         int64                          // view of PBFT consensus.
	stage        int                            // stage of PBFT consensus, containing preprepare, prepare, commit, view-change and new-view.
	curProposal  *message.PreprepareMsg         // preprepare message in this round.
	lastProposal *message.PreprepareMsg         // preprepare message in the last round.
	prepareSet   map[nodetopo.NodeInfo]struct{} // prepareSet collects the nodes sending prepare message.
	commitSet    map[nodetopo.NodeInfo]struct{} // commitSet collects the nodes sending commit message.
}

func newConsensusMeta(cfg config.ConsensusCfg, lp config.LocalParams) *consensusMeta {
	return &consensusMeta{
		cfg: cfg,

		info:   nodetopo.NodeInfo{NodeID: lp.NodeID, ShardID: lp.ShardID},
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
	if c.curProposal == nil {
		ppMsgList := c.msgPool.ReadPreprepareMsg(c.view, c.seq)
		for _, ppMsg := range ppMsgList {
			if ppMsg.View != c.view || ppMsg.Seq != c.seq || ppMsg.NodeID != c.leader {
				// if the message is invalid (wrong seq/view or wrong leader), drop it
				continue
			}

			c.curProposal = ppMsg

			break
		}
	}

	if c.curProposal == nil {
		return
	}

	pMsgList := c.msgPool.ReadPrepareMsg(c.view, c.seq)
	for _, pMsg := range pMsgList {
		if pMsg.View != c.view || pMsg.Seq != c.seq || !bytes.Equal(pMsg.Digest, c.curProposal.Digest) {
			// if the message is invalid (wrong seq/view or wrong digest), drop it
			continue
		}

		c.prepareSet[nodetopo.NodeInfo{NodeID: pMsg.NodeID, ShardID: pMsg.ShardID}] = struct{}{}
	}

	cMsgList := c.msgPool.ReadCommitMsg(c.view, c.seq)
	for _, cMsg := range cMsgList {
		if cMsg.View != c.view || cMsg.Seq != c.seq || !bytes.Equal(cMsg.Digest, c.curProposal.Digest) {
			// if the message is invalid (wrong seq/view or wrong digest), drop it
			continue
		}

		c.commitSet[nodetopo.NodeInfo{NodeID: cMsg.NodeID, ShardID: cMsg.ShardID}] = struct{}{}
	}
}

// step2Next make pbft metadata step to next stage, returns (oldStage, newStage, err)
func (c *consensusMeta) step2Next() (int, int, error) {
	switch c.stage {
	case stagePreprepare:
		if c.curProposal == nil {
			slog.Info("waiting for preprepare message")
			return stagePreprepare, stagePreprepare, nil
		}

		c.stage = stagePrepare

		return stagePreprepare, stagePrepare, nil

	case stagePrepare:
		if len(c.prepareSet) < int(2*c.f+1) {
			slog.Info("waiting for prepare message", "current", 2*c.f+1, "expect", len(c.prepareSet))
			return stagePrepare, stagePrepare, nil
		}

		c.stage = stageCommit

		return stagePrepare, stageCommit, nil

	case stageCommit:
		if len(c.commitSet) < int(2*c.f+1) {
			slog.Info("waiting for commit message", "current", 2*c.f+1, "expect", len(c.commitSet))
			return stageCommit, stageCommit, nil
		}

		// step into next round
		c.stage = stagePreprepare
		c.lastProposal = c.curProposal
		c.curProposal = nil
		c.prepareSet = map[nodetopo.NodeInfo]struct{}{}
		c.commitSet = map[nodetopo.NodeInfo]struct{}{}
		c.proposed = false
		c.seq++

		return stageCommit, stagePreprepare, nil
	}

	return 0, -1, fmt.Errorf("invalid stage %d", c.stage)
}
