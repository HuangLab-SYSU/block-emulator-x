package outsideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
)

// ShardOutsideMsgHandler describes the operation handler for messages out of the current shard.
type ShardOutsideMsgHandler interface {
	// HandleMsgOutsideShard handles messages outside this shard.
	HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error
}
