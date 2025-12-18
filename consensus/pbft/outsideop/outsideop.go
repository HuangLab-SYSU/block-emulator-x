package outsideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
)

type ShardOutsideMsgHandler interface {
	HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error
	Close()
}
