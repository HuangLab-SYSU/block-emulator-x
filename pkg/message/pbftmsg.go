package message

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
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

func (p *Proposal) Hash() ([]byte, error) {
	b, err := rlp.EncodeToBytes(p)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(b)

	return sum[:], nil
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
