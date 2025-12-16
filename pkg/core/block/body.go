package block

import (
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

// Body is the struct for transaction handling.
// Note that either MigrationOpt or Body is nil.
type Body struct {
	TxList []transaction.Transaction
}

// MigrationOpt is the struct for account migration.
// It saves the information of accounts that are to be migrated to this shard.
// Note that either MigrationOpt or Body is nil.
type MigrationOpt struct {
	MigratedAccounts []account.Address // MigratedAccounts is the list of accounts to be migrated in this stage.
	MigratedStates   []account.State   // MigratedStates is the list of account states corresponding to accounts in MigratedAccounts.
}
