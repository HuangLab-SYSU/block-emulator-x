package outsideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type ShardOutsideMsgOp interface {
	HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error
}
