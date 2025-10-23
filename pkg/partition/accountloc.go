package partition

import (
	"encoding/binary"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

// DefaultAccountLoc generates an account state with some balances.
func DefaultAccountLoc(a account.Address, shardNum int64) int64 {
	u32 := binary.BigEndian.Uint32(a[len(a)-4:])
	sid := int64(u32) % shardNum

	return sid
}
