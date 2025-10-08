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

	t.Run("push 3 types of messages in this round", func(t *testing.T) {
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
	})

	t.Run("push 3 types of messages in the next round", func(t *testing.T) {
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
	})

	t.Run("read msg in this round", func(t *testing.T) {
		ppMsg := mp.ReadPreprepareMsg(testView, testSequence)
		require.Len(t, ppMsg, 1)
		for _, p := range ppMsg {
			require.Equal(t, p.Digest, []byte(testThisRoundDigest))
			require.Equal(t, p.Seq, testSequence)
			require.Equal(t, p.View, testView)
		}

		pMsg := mp.ReadPrepareMsg(testView, testSequence)
		require.Len(t, pMsg, msgNumber)
		for _, p := range pMsg {
			require.Equal(t, p.Digest, []byte(testThisRoundDigest))
			require.Equal(t, p.Seq, testSequence)
			require.Equal(t, p.View, testView)
		}

		cMsg := mp.ReadCommitMsg(testView, testSequence)
		require.Len(t, cMsg, msgNumber)
		for _, c := range cMsg {
			require.Equal(t, c.Digest, []byte(testThisRoundDigest))
			require.Equal(t, c.Seq, testSequence)
			require.Equal(t, c.View, testView)
		}
	})

	t.Run("read msg in next view / round", func(t *testing.T) {
		// given a larger view, all testSequence with less view will be popped.
		ppMsg := mp.ReadPreprepareMsg(testView+1, testSequence)
		require.Len(t, ppMsg, 1)
		for _, p := range ppMsg {
			require.Equal(t, p.Digest, []byte(testNextRoundDigest))
			require.Equal(t, p.Seq, testSequence+1)
			require.Equal(t, p.View, testView)
		}

		pMsg := mp.ReadPrepareMsg(testView+1, testSequence)
		require.Len(t, pMsg, msgNumber)
		for _, p := range pMsg {
			require.Equal(t, p.Digest, []byte(testNextRoundDigest))
			require.Equal(t, p.Seq, testSequence+1)
			require.Equal(t, p.View, testView)
		}

		cMsg := mp.ReadCommitMsg(testView+1, testSequence)
		require.Len(t, cMsg, msgNumber)
		for _, c := range cMsg {
			require.Equal(t, c.Digest, []byte(testNextRoundDigest))
			require.Equal(t, c.Seq, testSequence+1)
			require.Equal(t, c.View, testView)
		}
	})

	t.Run("empty msg", func(t *testing.T) {
		ppMsg := mp.ReadPreprepareMsg(testView+1, testSequence+1)
		require.Len(t, ppMsg, 0)
		pMsg := mp.ReadPrepareMsg(testView+1, testSequence+1)
		require.Len(t, pMsg, 0)
		cMsg := mp.ReadCommitMsg(testView+1, testSequence+1)
		require.Len(t, cMsg, 0)
	})
}
