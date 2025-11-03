package committee

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type Committee interface {
	SendTxsAndConsensus(ctx context.Context) error
	HandleMsg(ctx context.Context, msg *rpcserver.WrappedMsg) error
	ShouldStop() bool
}

const stopThresholdPerShard = 5

type stopLogic struct {
	stopThreshold int64
	stopCnt       int64
}
