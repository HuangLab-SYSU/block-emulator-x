package pool

import (
	"testing"

	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/message"
	"github.com/stretchr/testify/require"
)

const (
	testSequence        = int64(1)
	testView            = int64(1)
	testThisRoundDigest = "test-this-round-block-digest"
	testNextRoundDigest = "test-next-round-block-digest"
	msgNumber           = 3
)

func TestPool(t *testing.T) {
	mp := NewMsgPool()

	msgPush(mp)
	testMsgReadThisRound(t, mp)
	testMsgReadNextView(t, mp)
	testMsgReadEmpty(t, mp)
}

func msgPush(mp *MsgPool) {
	// push msg in this round
	mp.PushPreprepareMsg(&message.PreprepareMsg{
		Digest: []byte(testThisRoundDigest),
		Seq:    testSequence,
		View:   testView,
		NodeID: 0,
	})

	for i := range msgNumber {
		mp.PushPrepareMsg(&message.PrepareMsg{
			Digest: []byte(testThisRoundDigest),
			Seq:    testSequence,
			View:   testView,
			NodeID: int64(i),
		})

		mp.PushCommitMsg(&message.CommitMsg{
			Digest: []byte(testThisRoundDigest),
			Seq:    testSequence,
			View:   testView,
			NodeID: int64(i),
		})
	}

	// push msg in the next round
	mp.PushPreprepareMsg(&message.PreprepareMsg{
		Digest: []byte(testNextRoundDigest),
		Seq:    testSequence + 1,
		View:   testView,
		NodeID: 0,
	})

	for i := range msgNumber {
		mp.PushPrepareMsg(&message.PrepareMsg{
			Digest: []byte(testNextRoundDigest),
			Seq:    testSequence + 1,
			View:   testView,
			NodeID: int64(i),
		})

		mp.PushCommitMsg(&message.CommitMsg{
			Digest: []byte(testNextRoundDigest),
			Seq:    testSequence + 1,
			View:   testView,
			NodeID: int64(i),
		})
	}
}

func testMsgReadThisRound(t *testing.T, mp *MsgPool) {
	ppMsg := mp.ReadPreprepareMsg(testView, testSequence)
	require.Len(t, ppMsg, 1)
	for _, p := range ppMsg {
		require.Equal(t, []byte(testThisRoundDigest), p.Digest)
		require.Equal(t, testSequence, p.Seq)
		require.Equal(t, testView, p.View)
	}

	pMsg := mp.ReadPrepareMsg(testView, testSequence)
	require.Len(t, pMsg, msgNumber)
	for _, p := range pMsg {
		require.Equal(t, []byte(testThisRoundDigest), p.Digest)
		require.Equal(t, testSequence, p.Seq)
		require.Equal(t, testView, p.View)
	}

	cMsg := mp.ReadCommitMsg(testView, testSequence)
	require.Len(t, cMsg, msgNumber)
	for _, c := range cMsg {
		require.Equal(t, []byte(testThisRoundDigest), c.Digest)
		require.Equal(t, testSequence, c.Seq)
		require.Equal(t, testView, c.View)
	}
}

func testMsgReadNextView(t *testing.T, mp *MsgPool) {
	// given a larger view, all testSequence with less view will be popped.
	ppMsg := mp.ReadPreprepareMsg(testView+1, testSequence)
	require.Len(t, ppMsg, 1)
	for _, p := range ppMsg {
		require.Equal(t, []byte(testNextRoundDigest), p.Digest)
		require.Equal(t, testSequence+1, p.Seq)
		require.Equal(t, testView, p.View)
	}

	pMsg := mp.ReadPrepareMsg(testView+1, testSequence)
	require.Len(t, pMsg, msgNumber)
	for _, p := range pMsg {
		require.Equal(t, []byte(testNextRoundDigest), p.Digest)
		require.Equal(t, testSequence+1, p.Seq)
		require.Equal(t, testView, p.View)
	}

	cMsg := mp.ReadCommitMsg(testView+1, testSequence)
	require.Len(t, cMsg, msgNumber)
	for _, c := range cMsg {
		require.Equal(t, []byte(testNextRoundDigest), c.Digest)
		require.Equal(t, testSequence+1, c.Seq)
		require.Equal(t, testView, c.View)
	}
}

func testMsgReadEmpty(t *testing.T, mp *MsgPool) {
	ppMsg := mp.ReadPreprepareMsg(testView+1, testSequence+1)
	require.Len(t, ppMsg, 0)
	pMsg := mp.ReadPrepareMsg(testView+1, testSequence+1)
	require.Len(t, pMsg, 0)
	cMsg := mp.ReadCommitMsg(testView+1, testSequence+1)
	require.Len(t, cMsg, 0)
}
