package message

import (
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

const (
	CLPARepartitionStartMessageType  = "CLPARepartitionStart"
	AccountAndTxMigrationMessageType = "AccountAndTxMigration"
)

type CLPARepartitionStartMsg struct {
	Epoch       int64
	ModifiedMap map[account.Account]int
}

type AccountAndTxMigrationMsg struct {
	SrcShard, DestShard int64
	Epoch               int64
	AccountStates       map[account.Account]*account.State
	MigratedTxs         []transaction.Transaction
}
