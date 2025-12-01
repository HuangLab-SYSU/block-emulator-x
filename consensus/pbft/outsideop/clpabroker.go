package outsideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type CLPABrokerOutsideOp struct{}

func (c *CLPABrokerOutsideOp) HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	// TODO implement me
	panic("implement me")
}

func (c *CLPABrokerOutsideOp) Close() {}
