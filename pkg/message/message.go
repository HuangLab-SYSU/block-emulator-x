package message

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

// WrapMsg encodes different types of messages.
// Note that this function's input should be a pointer
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
	case *StopConsensusMsg:
		msgType = StopConsensusMessageType

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
	case *BrokerCLPATxSendAgainMsg:
		msgType = BrokerCLPATxSendAgainMessageType

	case *CLPARepartitionStartMsg:
		msgType = CLPARepartitionStartMessageType

	default:
		return "", fmt.Errorf("unknown msg type: %T", msg)
	}

	return msgType, nil
}
