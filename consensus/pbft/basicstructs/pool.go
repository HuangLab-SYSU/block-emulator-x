package basicstructs

import (
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
)

// MsgPool contains 3 priority queues for PBFT messages, i.e., preprepare, prepare and commit, respectively.
type MsgPool struct {
	preprepares *PriorityQueue[*message.PreprepareMsg]
	prepares    *PriorityQueue[*message.PrepareMsg]
	commits     *PriorityQueue[*message.CommitMsg]
}

func NewMsgPool() *MsgPool {
	return &MsgPool{
		preprepares: NewPQ[*message.PreprepareMsg](PreprepareMsgLess),
		prepares:    NewPQ[*message.PrepareMsg](PrepareMsgLess),
		commits:     NewPQ[*message.CommitMsg](CommitMsgLess),
	}
}

func (m *MsgPool) ReadPreprepareMsg(end ViewSeq) []*message.PreprepareMsg {
	ret := make([]*message.PreprepareMsg, 0)

	for m.preprepares.Len() > 0 {
		top := m.preprepares.PopItem()
		if end.Compare(ViewSeq{View: top.View, Seq: top.Seq}) == -1 {
			m.preprepares.PushItem(top)
			break
		}

		ret = append(ret, top)
	}

	if len(ret) > 0 {
		slog.Debug(
			"read PreprepareMsg successfully",
			"end view and seq",
			end,
			"fetched size",
			len(ret),
			"rest size",
			m.preprepares.Len(),
		)
	}

	return ret
}

func (m *MsgPool) ReadPrepareMsg(end ViewSeq) []*message.PrepareMsg {
	ret := make([]*message.PrepareMsg, 0)

	for m.prepares.Len() > 0 {
		top := m.prepares.PopItem()
		if end.Compare(ViewSeq{View: top.View, Seq: top.Seq}) == -1 {
			m.prepares.PushItem(top)
			break
		}

		ret = append(ret, top)
	}

	if len(ret) > 0 {
		slog.Debug(
			"read PrepareMsg successfully",
			"end view and seq",
			end,
			"fetched size",
			len(ret),
			"rest size",
			m.preprepares.Len(),
		)
	}

	return ret
}

func (m *MsgPool) ReadCommitMsg(end ViewSeq) []*message.CommitMsg {
	ret := make([]*message.CommitMsg, 0)

	for m.commits.Len() > 0 {
		top := m.commits.PopItem()
		if end.Compare(ViewSeq{View: top.View, Seq: top.Seq}) == -1 {
			m.commits.PushItem(top)
			break
		}

		ret = append(ret, top)
	}

	if len(ret) > 0 {
		slog.Debug(
			"read CommitMsg successfully",
			"end view and seq",
			end,
			"fetched size",
			len(ret),
			"rest size",
			m.preprepares.Len(),
		)
	}

	return ret
}

func (m *MsgPool) PushPreprepareMsg(ppMsg *message.PreprepareMsg) {
	m.preprepares.PushItem(ppMsg)
}

func (m *MsgPool) PushPrepareMsg(pMsg *message.PrepareMsg) {
	m.prepares.PushItem(pMsg)
}

func (m *MsgPool) PushCommitMsg(cMsg *message.CommitMsg) {
	m.commits.PushItem(cMsg)
}
