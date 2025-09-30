package message

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

const (
	PreprepareMessageType = "Preprepare"
	PrepareMessageType    = "Prepare"
	CommitMessageType     = "Commit"

	ReceiveTxsMessageType = "ReceiveTxs"
)

// PreprepareMsg is the pre-prepare message in the PBFT consensus, and it contains a block and its digest (i.e., Hash).
type PreprepareMsg struct {
	B         block.Block
	Digest    []byte
	Seq, View int64
}

// PrepareMsg is the prepare message in the PBFT consensus, and it contains the digest and the agreement ack (always true).
type PrepareMsg struct {
	Digest    []byte
	Seq, View int64
}

// CommitMsg is the commit message in the PBFT consensus, and it contains the digest and the agreement ack (always true).
type CommitMsg struct {
	Digest    []byte
	Seq, View int64
}

type ReceiveTxsMsg struct {
	Txs []transaction.Transaction
}

// WrapMsg encodes different types of messages.
func WrapMsg(msg any) (*rpcserver.WrappedMsg, error) {
	var msgType string

	switch msg.(type) {
	case *PreprepareMsg:
		msgType = PreprepareMessageType
	case *PrepareMsg:
		msgType = PrepareMessageType
	case *CommitMsg:
		msgType = CommitMessageType
	case *ReceiveTxsMsg:
		msgType = ReceiveTxsMessageType
	default:
		return nil, fmt.Errorf("unknown msg type: %T", msg)
	}

	var buf bytes.Buffer

	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(msg)
	if err != nil {
		return nil, fmt.Errorf("encode failed for message: %w", err)
	}

	return &rpcserver.WrappedMsg{
		MsgType: msgType,
		Payload: buf.Bytes(),
	}, nil
}
