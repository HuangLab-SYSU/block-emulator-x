package committee

import (
	"context"

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
