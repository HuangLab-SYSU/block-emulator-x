package pool

import (
	"testing"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/stretchr/testify/require"
)

func TestPrepreparePQ_OrderByViewAndSeq(t *testing.T) {
	pq := NewPQ[*message.PreprepareMsg](PreprepareMsgLess)

	msgList := []*message.PreprepareMsg{
		{View: 2, Seq: 10},
		{View: 1, Seq: 5},
		{View: 1, Seq: 3},
		{View: 2, Seq: 8},
	}

	for _, m := range msgList {
		pq.PushItem(m)
	}

	expected := []*message.PreprepareMsg{
		{View: 1, Seq: 3},
		{View: 1, Seq: 5},
		{View: 2, Seq: 8},
		{View: 2, Seq: 10},
	}

	got := make([]*message.PreprepareMsg, 0, len(msgList))
	for pq.Len() > 0 {
		got = append(got, pq.PopItem())
	}
	require.Equal(t, expected, got)
}

func TestPrepreparePQ_PushPopSingle(t *testing.T) {
	pq := NewPQ[*message.PreprepareMsg](PreprepareMsgLess)
	item := &message.PreprepareMsg{View: 3, Seq: 1}
	pq.PushItem(item)
	require.Len(t, pq.items, 1)

	got := pq.PopItem()
	require.Equal(t, item, got)
}

func TestPrepreparePQ_SameViewDifferentSeq(t *testing.T) {
	pq := NewPQ[*message.PreprepareMsg](PreprepareMsgLess)
	pq.PushItem(&message.PreprepareMsg{View: 5, Seq: 20})
	pq.PushItem(&message.PreprepareMsg{View: 5, Seq: 10})
	pq.PushItem(&message.PreprepareMsg{View: 5, Seq: 30})

	expectedSeq := []int64{10, 20, 30}
	for i := 0; pq.Len() > 0; i++ {
		item := pq.PopItem()
		require.Equal(t, expectedSeq[i], item.Seq)
	}
}
