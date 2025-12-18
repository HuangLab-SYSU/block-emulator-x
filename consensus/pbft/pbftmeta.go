package pbft

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/basicstructs"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

const (
	initialLeader = 0

	stagePreprepare = 0
	stagePrepare    = 1
	stageCommit     = 2
	stageNewView    = 3
	stageViewChange = 4
)

const (
	// catchUpThreshold is the view-interval threshold to trigger the Catch-Up mechanism in PBFT.
	// If the view of this node is catchUpThreshold behind the latest proposal, this node will use Catch-Up.
	catchUpThreshold = 5
)

// consensusMeta is the metadata of PBFT consensus.
// Note that, consensusMeta is not thread-safe.
type consensusMeta struct {
	cfg config.ConsensusNodeCfg
	lp  config.LocalParams

	f      int64 // the number of fault-tolerance nodes
	leader int64 // the leader id
	closed bool  // consensus is closed or not

	msgPool *basicstructs.MsgPool

	// metadata for a single round;
	// variables below will update per round.
	proposed        bool                           // this node has proposed in this round or not.
	curViewSeq      basicstructs.ViewSeq           // curViewSeq is current pbft view and sequence.
	stage           int                            // stage of PBFT consensus, containing preprepare, prepare, commit, view-change and new-view.
	curProposal     *message.PreprepareMsg         // preprepare message in this round.
	lastProposal    *message.PreprepareMsg         // preprepare message in the last round.
	lastProposeTime time.Time                      // the time for the last proposal.
	prepareSet      map[nodetopo.NodeInfo]struct{} // prepareSet collects the nodes sending prepare message.
	commitSet       map[nodetopo.NodeInfo]struct{} // commitSet collects the nodes sending commit message.

	// variables to catch-up proposals
	latestViewSeq  basicstructs.ViewSeq // latestViewSeq is the latest view and seq of proposals in the message pool.
	catchupStarted bool                 // catchupStarted shows whether the catchup mechanism is started.
}

func newConsensusMeta(cfg config.ConsensusNodeCfg, lp config.LocalParams) *consensusMeta {
	return &consensusMeta{
		cfg: cfg,
		lp:  lp,

		f:      (cfg.NodeNum - 1) / 3,
		leader: initialLeader,
		closed: false,

		msgPool: basicstructs.NewMsgPool(),

		proposed:        false,
		curViewSeq:      basicstructs.ViewSeq{},
		stage:           stagePreprepare,
		curProposal:     nil,
		lastProposal:    nil,
		lastProposeTime: time.Now(),
		prepareSet:      map[nodetopo.NodeInfo]struct{}{},
		commitSet:       map[nodetopo.NodeInfo]struct{}{},

		latestViewSeq:  basicstructs.ViewSeq{},
		catchupStarted: false,
	}
}

// curateMsg counts the number of valid messages in the msgPool.
func (c *consensusMeta) curateMsg() {
	if c.curProposal == nil {
		ppMsgList := c.msgPool.ReadPreprepareMsg(c.curViewSeq)
		for _, ppMsg := range ppMsgList {
			if c.curViewSeq.Compare(basicstructs.ViewSeq{View: ppMsg.View, Seq: ppMsg.Seq}) != 0 || ppMsg.NodeID != c.leader {
				// if the message is invalid (wrong seq/view or wrong leader), drop it
				slog.Info("get an invalid preprepare message, ignore it", "view", ppMsg.View, "seq", ppMsg.Seq)
				continue
			}

			slog.Info("proposal of this round is received", "current view and seq", c.curViewSeq)
			c.curProposal = ppMsg

			break
		}
	}

	if c.curProposal == nil {
		return
	}

	pMsgList := c.msgPool.ReadPrepareMsg(c.curViewSeq)
	for _, pMsg := range pMsgList {
		if c.curViewSeq.Compare(basicstructs.ViewSeq{View: pMsg.View, Seq: pMsg.Seq}) != 0 || !bytes.Equal(pMsg.Digest, c.curProposal.Digest) {
			// if the prepare message is invalid (wrong seq/view or wrong digest), drop it
			slog.Info("get an invalid prepare message, ignore it", "view", pMsg.View, "seq", pMsg.Seq)
			continue
		}

		c.prepareSet[nodetopo.NodeInfo{NodeID: pMsg.NodeID, ShardID: pMsg.ShardID}] = struct{}{}
	}

	cMsgList := c.msgPool.ReadCommitMsg(c.curViewSeq)
	for _, cMsg := range cMsgList {
		if c.curViewSeq.Compare(basicstructs.ViewSeq{View: cMsg.View, Seq: cMsg.Seq}) != 0 || !bytes.Equal(cMsg.Digest, c.curProposal.Digest) {
			// if the commit message is invalid (wrong seq/view or wrong digest), drop it
			slog.Info("get an invalid commit message, ignore it", "view", cMsg.View, "seq", cMsg.Seq)
			continue
		}

		c.commitSet[nodetopo.NodeInfo{NodeID: cMsg.NodeID, ShardID: cMsg.ShardID}] = struct{}{}
	}
}

// step2Next makes pbft metadata step to next stage, returns (oldStage, newStage, err)
func (c *consensusMeta) step2Next() (int, int, error) {
	switch c.stage {
	case stagePreprepare:
		if c.curProposal == nil {
			slog.Debug("waiting for preprepare message")
			return stagePreprepare, stagePreprepare, nil
		}

		c.stage = stagePrepare

		return stagePreprepare, stagePrepare, nil

	case stagePrepare:
		if len(c.prepareSet) < int(2*c.f+1) {
			slog.Debug("waiting for prepare message", "expect", 2*c.f+1, "current", len(c.prepareSet))
			return stagePrepare, stagePrepare, nil
		}

		c.stage = stageCommit

		return stagePrepare, stageCommit, nil

	case stageCommit:
		if len(c.commitSet) < int(2*c.f+1) {
			slog.Debug("waiting for commit message", "expect", 2*c.f+1, "current", len(c.commitSet))
			return stageCommit, stageCommit, nil
		}

		// step into next round
		c.stage = stagePreprepare
		c.lastProposal = c.curProposal
		c.curProposal = nil
		c.prepareSet = map[nodetopo.NodeInfo]struct{}{}
		c.commitSet = map[nodetopo.NodeInfo]struct{}{}
		c.proposed = false
		c.curViewSeq.Seq++

		return stageCommit, stagePreprepare, nil
	}

	return 0, -1, fmt.Errorf("invalid stage %d", c.stage)
}

// updateLatestViewSeq updates the latest view and seq in the message pool.
func (c *consensusMeta) updateLatestViewSeq(view, seq int64) {
	in := basicstructs.ViewSeq{View: view, Seq: seq}
	if c.latestViewSeq.Compare(in) == -1 {
		c.latestViewSeq = in
	}
}

// catchupReady returns whether the catch-up mechanism is ready to start.
func (c *consensusMeta) catchupReady() bool {
	if c.catchupStarted {
		// If the catch-up starts already, should not start a new one.
		return false
	}
	// If the view is unchanged and the seq is catchUpThreshold behind, return true.
	return c.latestViewSeq.View == c.curViewSeq.View && c.latestViewSeq.Seq >= c.curViewSeq.Seq+catchUpThreshold
}

func (c *consensusMeta) catchupOverAndReset(vs basicstructs.ViewSeq) {
	// Catchup over
	c.catchupStarted = false
	c.curViewSeq = vs

	// Reset
	c.stage = stagePreprepare
	c.lastProposal = nil
	c.curProposal = nil
	c.prepareSet = map[nodetopo.NodeInfo]struct{}{}
	c.commitSet = map[nodetopo.NodeInfo]struct{}{}
	c.proposed = false
}
