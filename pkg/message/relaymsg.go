package message

import (
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
)

const RelayBlockInfoMessageType = "RelayBlockInfo" // Consensus nodes (using transaction-relay to handle cross-shard tx) send this type of message to supervisor

type RelayBlockInfoMsg struct {
	InnerShardTxs, Relay1Txs, Relay2Txs []transaction.Transaction
	Epoch                               int64
	BlockProposeTime, BlockCommitTime   time.Time
	ShardID                             int64
}
