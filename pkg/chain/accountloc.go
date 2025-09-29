package chain

import (
	"encoding/binary"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

func accountDefaultShard(account account.Account, shardNum int64) int64 {
	u32 := binary.BigEndian.Uint32(account.Addr[len(account.Addr)-4:])
	return int64(u32) % shardNum
}
