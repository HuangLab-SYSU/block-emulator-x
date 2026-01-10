package committee

import (
	"context"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
)

// Committee is the interface for a committee / client.
// HandleMsg and SendTxsAndConsensus should be called in serial, thus they will be in a single goroutine.
type Committee interface {
	// SendTxsAndConsensus sends messages including transactions and consensus.
	SendTxsAndConsensus(ctx context.Context) error
	// HandleMsg handles the wrapped msg.
	HandleMsg(ctx context.Context, msg *rpcserver.WrappedMsg) error
	// ShouldStop shows that all messages are sent or not.
	ShouldStop() bool
}

const stopThresholdPerShard = 5

const (
	clpaWeightPenalty = 0.5
	clpaMaxIterations = 100
)

type stopLogic struct {
	stopThreshold int64
	stopCnt       int64
}

type txLocationFunc func(tx transaction.Transaction) int64

func packShardTxs(
	txs []transaction.Transaction,
	shardNumber int64,
	locFunc txLocationFunc,
) [][]transaction.Transaction {
	shardTxs := make([][]transaction.Transaction, shardNumber)

	// classify the transactions by the locations of sender
	for _, tx := range txs {
		shardID := locFunc(tx)
		if shardID < 0 || shardID >= shardNumber {
			slog.Info("packShardTxs gets invalid shardID by locFunc", "invalid shardID", shardID)
			continue
		}

		shardTxs[shardID] = append(shardTxs[shardID], tx)
	}

	return shardTxs
}

func transferMapBytes2Addr(src map[[20]byte]int) map[account.Address]int {
	dest := make(map[account.Address]int, len(src))
	for k, v := range src {
		dest[k] = v
	}

	return dest
}
