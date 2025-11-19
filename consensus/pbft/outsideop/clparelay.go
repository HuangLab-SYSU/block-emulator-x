package outsideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type CLPARelayOutsideOp struct{}

func (c *CLPARelayOutsideOp) HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	// TODO implement me
	panic("implement me")
}

func (c *CLPARelayOutsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
