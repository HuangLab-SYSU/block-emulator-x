package migration

import (
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
)

type AccMigrateMetadata struct {
	Epoch                 int64
	CurModifiedMap        map[account.Account]int
	MigratedAccountStates map[int64]map[account.Account]*account.State
	MigrationReady        bool

	// unhandledStateMsg records those unhandled messages. The key is the epoch id of the messages.
	unhandledStateMsg map[int64][]*message.AccountAndTxMigrationMsg

	cfg config.SystemCfg
	lp  config.LocalParams
}

func NewAccMigrateMetadata(cfg config.SystemCfg, lp config.LocalParams) *AccMigrateMetadata {
	return &AccMigrateMetadata{
		Epoch:                 0,
		CurModifiedMap:        make(map[account.Account]int),
		MigratedAccountStates: make(map[int64]map[account.Account]*account.State),
		MigrationReady:        false,
		unhandledStateMsg:     make(map[int64][]*message.AccountAndTxMigrationMsg),
		cfg:                   cfg,
		lp:                    lp,
	}
}

func (am *AccMigrateMetadata) MigrationStatusReset() {
	am.MigratedAccountStates = make(map[int64]map[account.Account]*account.State)
	am.CurModifiedMap = make(map[account.Account]int)
	am.MigrationReady = false
	am.unhandledStateMsg[am.Epoch] = nil
}

// UpdateByRepartitionStartMsg updates the AccMigrateMetadata by the given CLPARepartitionStartMsg.
// If there are unhandled messages (with the same epoch id) in the unhandledStateMsg,
// these messages will be handled by calling CollectStatesByMsg.
// **Because the network is async., the AccountAndTxMigrationMsg may be early arrived but CLPARepartitionStartMsg not.**
func (am *AccMigrateMetadata) UpdateByRepartitionStartMsg(cr *message.CLPARepartitionStartMsg) error {
	if cr.Epoch != am.Epoch+1 {
		return fmt.Errorf("wrong epoch ID in CLPARepartitionStartMsg, expect=%d, got=%d", am.Epoch+1, cr.Epoch)
	}

	am.Epoch = cr.Epoch // increase the epoch id
	am.CurModifiedMap = cr.ModifiedMap
	am.MigrationReady = true

	// handle the messages before
	for _, atMsg := range am.unhandledStateMsg[am.Epoch] {
		if err := am.CollectStatesByMsg(atMsg); err != nil {
			slog.Error("handle AccountAndTxMigrationMsg in unhandledStateMsg failed", "err", err)
		}
	}
	// clear this unhandled pool
	am.unhandledStateMsg[am.Epoch] = nil

	return nil
}

// CollectStatesByMsg collects the MigratedAccountStates according to the given message.
// Note that, if the message is newer than this AccMigrateMetadata, it will be added to unhandledStateMsg.
// **Because the network is async., the AccountAndTxMigrationMsg may be early arrived but CLPARepartitionStartMsg not.**
// When the AccMigrateMetadata reaches the epoch id, the messages in unhandledStateMsg will be handled.
func (am *AccMigrateMetadata) CollectStatesByMsg(atMsg *message.AccountAndTxMigrationMsg) error {
	if atMsg.DestShard != am.lp.ShardID {
		return fmt.Errorf("wrong dest shard ID=%d, current shard=%d", atMsg.DestShard, am.lp.ShardID)
	}

	if atMsg.SrcShard >= am.cfg.ShardNum || atMsg.SrcShard < 0 {
		return fmt.Errorf("invalid src shard ID=%d", atMsg.SrcShard)
	}

	if atMsg.Epoch < am.Epoch {
		return fmt.Errorf("out-of-date epoch ID in AccountAndTxMigrationMsg, expect=%d, got=%d", am.Epoch, atMsg.Epoch)
	}

	if atMsg.Epoch > am.Epoch {
		am.unhandledStateMsg[atMsg.Epoch] = append(am.unhandledStateMsg[atMsg.Epoch], atMsg)
		slog.Info("receives newer AccountAndTxMigrationMsg, epoch=%d, add it to buffer", "received epoch", atMsg.Epoch)

		return nil
	}

	am.MigratedAccountStates[atMsg.SrcShard] = atMsg.AccountStates

	return nil
}

// GetMigratedAccountsAndStates returns the migrated accounts and states if all MigratedAccountStates are collected.
func (am *AccMigrateMetadata) GetMigratedAccountsAndStates() ([]account.Account, []account.State, error) {
	if len(am.MigratedAccountStates) != int(am.cfg.ShardNum) {
		return nil, nil, fmt.Errorf("not all MigratedAccountStates is collected, expect=%d, got=%d", am.cfg.ShardNum, len(am.MigratedAccountStates))
	}

	accountMigratedIn := make(map[account.Account]*account.State, len(am.MigratedAccountStates))
	// merge MigratedAccountStates into one map
	for _, migratedStateMap := range am.MigratedAccountStates {
		for k, v := range migratedStateMap {
			accountMigratedIn[k] = v
		}
	}

	migratedAccounts := make([]account.Account, 0, len(am.MigratedAccountStates))

	migratedStates := make([]account.State, 0, len(am.MigratedAccountStates))

	for acc, destShardID := range am.CurModifiedMap {
		state := accountMigratedIn[acc]
		if state == nil {
			if int64(destShardID) == am.lp.ShardID {
				slog.Warn("missing account in GetMigratedAccountsAndStates", "account", acc)
			}

			state = account.NewState(acc, int64(destShardID))
		} else {
			// Set the location of this state to be this one.
			state.ShardLocation = int64(destShardID)
		}

		migratedAccounts = append(migratedAccounts, acc)
		migratedStates = append(migratedStates, *state)
	}

	return migratedAccounts, migratedStates, nil
}
