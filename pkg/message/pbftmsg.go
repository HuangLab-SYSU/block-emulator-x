package message

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

const (
	PreprepareMessageType = "Preprepare"
	PrepareMessageType    = "Prepare"
	CommitMessageType     = "Commit"

	ReceiveTxsMessageType = "ReceiveTxs"

	StopConsensusMessageType = "StopConsensus"
)

const (
	BlockProposalType     = "BlockProposal"
	PartitionProposalType = "PartitionProposalType"
)

type Proposal struct {
	ProposalType string
	Payload      []byte
}

func (p *Proposal) Encode() ([]byte, error) {
	var buff bytes.Buffer

	encoder := gob.NewEncoder(&buff)

	err := encoder.Encode(p)
	if err != nil {
		return nil, fmt.Errorf("encode state failed: %w", err)
	}

	return buff.Bytes(), nil
}

// PreprepareMsg is the pre-prepare message in the PBFT consensus, and it contains a block and its digest (i.e., Hash).
type PreprepareMsg struct {
	P               Proposal
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

type StopConsensusMsg struct {
	StopSignal struct{}
}
