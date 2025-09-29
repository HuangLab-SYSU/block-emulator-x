package message

import (
	"sync"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type MsgPool struct {
	mux sync.Mutex
}

func NewMsgPool() *MsgPool {
	return &MsgPool{}
}

func (m *MsgPool) ReadTop(endView, endSeq int64) []*rpcserver.WrappedMsg {
	m.mux.Lock()
	defer m.mux.Unlock()

	panic("implement me")
}

func (m *MsgPool) Put(msg []*rpcserver.WrappedMsg) {
	m.mux.Lock()
	defer m.mux.Unlock()

	panic("implement me")
}
