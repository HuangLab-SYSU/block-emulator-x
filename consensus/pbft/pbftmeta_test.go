package pbft

import (
	"testing"

	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/basicstructs"
	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
)

const (
	testDigest       = "test-block-digest"
	testNextDigest   = "test-block-next-digest"
	testSequence     = int64(0)
	testView         = int64(0)
	testLeaderNodeID = int64(0)
	testNodeNum      = int64(4)
)

var consensusCfg = config.ConsensusNodeCfg{
	BlockchainCfg: config.BlockchainCfg{
		SystemCfg: config.SystemCfg{
			ShardNum: 1,
			NodeNum:  testNodeNum,
		},
	},
}

var localParams = config.LocalParams{
	NodeID:  0,
	ShardID: 0,
}

func TestConsensusMeta(t *testing.T) {
	cm := newConsensusMeta(consensusCfg, localParams)

	// inject messages
	injectMsg(cm)

	_, _, err := cm.step2Next()
	require.NoError(t, err)

	require.Nil(t, cm.curPreprepare)
	cm.curateMsg()
	require.NotNil(t, cm.curPreprepare)
	require.Equal(t, []byte(testDigest), cm.curPreprepare.Digest)

	require.Equal(t, stagePreprepare, cm.stage)
	oldStage, stage, err := cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stagePreprepare, oldStage)
	require.Equal(t, stagePrepare, stage)
	require.Equal(t, stagePrepare, cm.stage)

	oldStage, stage, err = cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stagePrepare, oldStage)
	require.Equal(t, stageCommit, stage)
	require.Equal(t, stageCommit, cm.stage)

	oldStage, stage, err = cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stageCommit, oldStage)
	require.Equal(t, stagePreprepare, stage)
	require.Equal(t, stagePreprepare, cm.stage)
	require.Equal(t, testSequence+1, cm.curViewSeq.Seq)
	require.Nil(t, cm.curPreprepare)

	cm.curateMsg()
	_, stage, err = cm.step2Next()
	require.NoError(t, err)
	_, stage, err = cm.step2Next()
	require.NoError(t, err)
	_, stage, err = cm.step2Next()
	require.NoError(t, err)
	require.Equal(t, stagePreprepare, stage)
	require.Equal(t, stagePreprepare, cm.stage)
	require.Equal(t, testSequence+2, cm.curViewSeq.Seq)
	require.Nil(t, cm.curPreprepare)

	// Catchup.
	catchupViewSeq := basicstructs.ViewSeq{View: testView + 3, Seq: testSequence + 3}
	cm.updateLatestViewSeq(catchupViewSeq.View, catchupViewSeq.Seq)
	cm.catchupReady()
	cm.catchupOverAndReset(catchupViewSeq)
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
