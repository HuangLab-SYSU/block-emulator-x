package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

const (
	stateReasonAccountMigration = byte(31)
	stateReasonTransaction      = byte(30)
	stateReasonBalanceInit      = byte(29)
)

// readStateFromVMExecutor reads account state from the given executor.
// If the state does not exist, a default account state will be returned.
func readStateFromVMExecutor(address account.Address, e *vm.Executor, location uint64) *account.State {
	addr := common.Address(address)
	if !e.StateDB().Exist(addr) {
		return account.NewState(address, location)
	}

	balance := e.StateDB().GetBalance(addr)
	nonce := e.StateDB().GetNonce(addr)

	var bInt *big.Int
	balance.IntoBig(&bInt)

	return &account.State{
		Address: address,
		Nonce:   nonce,
		Balance: bInt,

		ShardLocation: location,
	}
}

// setMigratedStates2VMTrie writes states of EOA to the trie database by vm.Executor.
func setMigratedStates2VMTrie(address account.Address, s account.State, e *vm.Executor) error {
	addr := common.Address(address)

	b, err := utils.BigToUInt256(s.Balance)
	if err != nil {
		return fmt.Errorf("big to string err: %w", err)
	}

	e.StateDB().SetBalance(addr, b, tracing.BalanceChangeReason(stateReasonAccountMigration))
	e.StateDB().SetNonce(addr, s.Nonce, tracing.NonceChangeReason(stateReasonAccountMigration))

	return nil
}
