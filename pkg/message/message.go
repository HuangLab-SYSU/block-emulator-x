package message

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
)

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

func WrapProposal(b *block.Block, proposalType string) (*Proposal, error) {
	var p Proposal

	payload, err := b.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode block payload: %w", err)
	}

	p.Payload = payload
	p.ProposalType = proposalType

	return &p, nil
}

func getMsgType(msg any) (string, error) {
	var msgType string

	switch msg.(type) {
	case *StopConsensusMsg, StopConsensusMsg:
		msgType = StopConsensusMessageType

	case *PreprepareMsg, PreprepareMsg:
		msgType = PreprepareMessageType
	case *PrepareMsg, PrepareMsg:
		msgType = PrepareMessageType
	case *CommitMsg, CommitMsg:
		msgType = CommitMessageType
	case *ReceiveTxsMsg, ReceiveTxsMsg:
		msgType = ReceiveTxsMessageType
	case *RelayBlockInfoMsg, RelayBlockInfoMsg:
		msgType = RelayBlockInfoMessageType

	case *BrokerBlockInfoMsg, BrokerBlockInfoMsg:
		msgType = BrokerBlockInfoMessageType
	case *BrokerCLPATxSendAgainMsg, BrokerCLPATxSendAgainMsg:
		msgType = BrokerCLPATxSendAgainMessageType

	case *CLPARepartitionStartMsg, CLPARepartitionStartMsg:
		msgType = CLPARepartitionStartMessageType
	case *AccountMigrationMsg, AccountMigrationMsg:
		msgType = AccountMigrationMessageType

	default:
		return "", fmt.Errorf("unknown msg type: %T", msg)
	}

	return msgType, nil
}
