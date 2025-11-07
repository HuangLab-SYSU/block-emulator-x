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
	B               block.Block
	Digest          []byte
	Seq, View       int64
	ShardID, NodeID int64
}

// PrepareMsg is the prepare message in the PBFT consensus, and it contains the digest and the agreement ack (always true).
type PrepareMsg struct {
	Digest          []byte
	Seq, View       int64
	ShardID, NodeID int64
}

// CommitMsg is the commit message in the PBFT consensus, and it contains the digest and the agreement ack (always true).
type CommitMsg struct {
	Digest          []byte
	Seq, View       int64
	ShardID, NodeID int64
}

type ReceiveTxsMsg struct {
	Txs []transaction.Transaction
}

// WrapMsg encodes different types of messages.
func WrapMsg(msg any) (*rpcserver.WrappedMsg, error) {
	msgType, err := getMsgType(msg)
	if err != nil {
		return nil, fmt.Errorf("getMsgType failed: %w", err)
	}

	var buf bytes.Buffer

	encoder := gob.NewEncoder(&buf)

	if err = encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("encoder failed: %w", err)
	}

	return &rpcserver.WrappedMsg{
		MsgType: msgType,
		Payload: buf.Bytes(),
	}, nil
}

func getMsgType(msg any) (string, error) {
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
	case *RelayBlockInfoMsg:
		msgType = RelayBlockInfoMessageType
	case *BrokerBlockInfoMsg:
		msgType = BrokerBlockInfoMessageType
	case *CLPARepartitionStartMsg:
		msgType = CLPARepartitionStartMessageType
	default:
		return "", fmt.Errorf("unknown msg type: %T", msg)
	}

	return msgType, nil
}
