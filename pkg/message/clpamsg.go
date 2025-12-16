package message

import (
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

const (
	CLPARepartitionStartMessageType = "CLPARepartitionStart"
	AccountMigrationMessageType     = "AccountMigration"
)

type CLPARepartitionStartMsg struct {
	Epoch       int64
	ModifiedMap map[account.Address]int
}

type AccountMigrationMsg struct {
	SrcShard, DestShard int64
	Epoch               int64
	AccountStates       map[account.Address]*account.State
}
