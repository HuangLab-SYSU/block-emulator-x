package pool

import (
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

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

func (m *MsgPool) ReadPreprepareMsg(endView, endSeq int64) []*message.PreprepareMsg {
	ret := make([]*message.PreprepareMsg, 0)

	for m.preprepares.Len() > 0 {
		top := m.preprepares.PopItem()
		if top.View > endView || (top.View == endView && top.Seq > endSeq) {
			m.preprepares.PushItem(top)
			break
		}

		ret = append(ret, top)
	}

	return ret
}

func (m *MsgPool) ReadPrepareMsg(endView, endSeq int64) []*message.PrepareMsg {
	ret := make([]*message.PrepareMsg, 0)

	for m.prepares.Len() > 0 {
		top := m.prepares.PopItem()
		if top.View > endView || (top.View == endView && top.Seq > endSeq) {
			m.prepares.PushItem(top)
			break
		}

		ret = append(ret, top)
	}

	return ret
}

func (m *MsgPool) ReadCommitMsg(endView, endSeq int64) []*message.CommitMsg {
	ret := make([]*message.CommitMsg, 0)

	for m.commits.Len() > 0 {
		top := m.commits.PopItem()
		if top.View > endView || (top.View == endView && top.Seq > endSeq) {
			m.commits.PushItem(top)
			break
		}

		ret = append(ret, top)
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
