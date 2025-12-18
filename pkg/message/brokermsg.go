package message

import (
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
)

const (
	BrokerBlockInfoMessageType       = "BrokerBlockInfo"       // Consensus nodes (using broker-transaction to handle cross-shard tx) send this type of message to supervisor
	BrokerCLPATxSendAgainMessageType = "BrokerCLPATxSendAgain" // In CLPA+Broker, inner-shard txs may become a cross-shard one because of the account-repartition operation.
)

type BrokerBlockInfoMsg struct {
	InnerShardTxs, Broker1Txs, Broker2Txs []transaction.Transaction
	Epoch                                 int64
	BlockProposeTime, BlockCommitTime     time.Time
	ShardID                               int64
}

type BrokerCLPATxSendAgainMsg struct {
	Txs []transaction.Transaction
}
