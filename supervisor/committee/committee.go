package committee

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

// Committee is the interface for a committee.
// HandleMsg and SendTxsAndConsensus should be called in serial, thus they should be in a thread-safe goroutine.
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

func PackShardTxs(txs []transaction.Transaction, shardNumber int64, locFunc txLocationFunc) (map[int]*rpcserver.WrappedMsg, error) {
	shardTxs := make([][]transaction.Transaction, shardNumber)

	// classify the transactions by the locations of sender
	for _, tx := range txs {
		shardID := locFunc(tx)
		shardTxs[shardID] = append(shardTxs[shardID], tx)
	}

	msg2Shard := make(map[int]*rpcserver.WrappedMsg, shardNumber)

	for i := range shardTxs {
		w, err := message.WrapMsg(message.ReceiveTxsMsg{
			Txs: shardTxs[i],
		})
		if err != nil {
			return nil, fmt.Errorf("failed to wrap message: %w", err)
		}

		msg2Shard[i] = w
	}

	return msg2Shard, nil
}
