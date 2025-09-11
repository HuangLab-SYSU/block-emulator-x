package chain

import (
	"encoding/binary"

	"github.com/HuangLab-SYSU/block-emulator/core/account"
)

func accountDefaultShard(account account.Account, shardNum int64) int64 {
	var buf [8]byte
	copy(buf[4:], account.Addr[16:]) // 4 bytes
	u32 := binary.BigEndian.Uint32(buf[:])
	return int64(u32) % shardNum
}
