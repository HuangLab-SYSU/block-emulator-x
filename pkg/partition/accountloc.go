package partition

import (
	"encoding/binary"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
)

// DefaultAccountLoc returns the shard location of the given account.
func DefaultAccountLoc(a account.Address, shardNum int64) int64 {
	u32 := binary.BigEndian.Uint32(a[len(a)-4:])
	sid := int64(u32) % shardNum

	return sid
}
