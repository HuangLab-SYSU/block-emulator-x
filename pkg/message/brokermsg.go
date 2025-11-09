package message

import (
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

const (
	BrokerBlockInfoMessageType = "BrokerBlockInfo" // Consensus nodes (using broker-transaction to handle cross-shard tx) send this type of message to supervisor
)

type BrokerBlockInfoMsg struct {
	InnerShardTxs, Broker1Txs, Broker2Txs []transaction.Transaction
	Epoch                                 int64
	BlockProposeTime, BlockCommitTime     time.Time
	ShardID                               int64
}
