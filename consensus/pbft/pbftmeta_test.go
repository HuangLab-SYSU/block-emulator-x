package pbft

import (
	"testing"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/message"
	"github.com/stretchr/testify/require"
)

const (
	testDigest       = "test-block-digest"
	testNextDigest   = "test-block-next-digest"
	testSequence     = int64(0)
	testView         = int64(0)
	testLeaderNodeID = int64(0)
	testNodeNum      = int64(4)
)

var consensusCfg = config.ConsensusCfg{
	ShardNum:          1,
	NodeNum:           testNodeNum,
	HandlerBufferSize: 1 << 10,
	BlockInterval:     int64(1 * time.Second),
}

func TestConsensusMeta(t *testing.T) {
	cm := newConsensusMeta(consensusCfg)

	// inject messages
	injectMsg(cm)

	require.Nil(t, cm.curProposal)
	cm.curateMsg()
	require.NotNil(t, cm.curProposal)
	require.Equal(t, []byte(testDigest), cm.curProposal.Digest)

	require.Equal(t, stagePreprepare, cm.stage)
	stage, err := cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stagePrepare, stage)
	require.Equal(t, stagePrepare, cm.stage)

	stage, err = cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stageCommit, stage)
	require.Equal(t, stageCommit, cm.stage)

	stage, err = cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stagePreprepare, stage)
	require.Equal(t, stagePreprepare, cm.stage)
	require.Equal(t, testSequence+1, cm.seq)
	require.Nil(t, cm.curProposal)

	cm.curateMsg()
	stage, err = cm.step2Next()
	require.NoError(t, err)
	stage, err = cm.step2Next()
	require.NoError(t, err)
	stage, err = cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stagePreprepare, stage)
	require.Equal(t, stagePreprepare, cm.stage)
	require.Equal(t, testSequence+2, cm.seq)
	require.Nil(t, cm.curProposal)
}

func injectMsg(cm *consensusMeta) {
	seq2Digest := map[int64][]byte{testSequence: []byte(testDigest), testSequence + 1: []byte(testNextDigest)}
	for seq, dig := range seq2Digest {
		cm.msgPool.PushPreprepareMsg(&message.PreprepareMsg{
			Digest: dig,
			Seq:    seq,
			View:   testView,
			NodeID: testLeaderNodeID,
		})

		for i := range testNodeNum {
			cm.msgPool.PushPrepareMsg(&message.PrepareMsg{
				Digest: dig,
				Seq:    seq,
				View:   testView,
				NodeID: i,
			})

			cm.msgPool.PushCommitMsg(&message.CommitMsg{
				Digest: dig,
				Seq:    seq,
				View:   testView,
				NodeID: i,
			})
		}
	}
}
