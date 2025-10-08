package chain

import (
	"encoding/binary"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

// generateInitAccountState generates an account state with some balances.
func generateInitAccountState(a account.Account, shardNum int64) *account.State {
	u32 := binary.BigEndian.Uint32(a.Addr[len(a.Addr)-4:])
	sid := int64(u32) % shardNum
	s := account.NewState(a, []int64{sid})

	return s
}
