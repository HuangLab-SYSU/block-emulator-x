package vm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
)

const EIP3855 = 3855

// Executor wraps the functions of stateDB and evm of geth.
// An executor should only be used in a block (i.e., BlockContext).
// After updating the stateDB, the executor should be committed to save the modifications.
// Note that, a new executor should be created to handle the transactions in a following block.
type Executor struct {
	stateDB    *state.StateDB
	vmChainCfg *params.ChainConfig
	evmCfg     gethvm.Config
}

// NewExecutor creates a new executor with given parameters.
func NewExecutor(stateStore state.Database, root common.Hash, vmChainCfg *params.ChainConfig) (*Executor, error) {
	// Init state db.
	stateDB, err := state.New(root, stateStore)
	if err != nil {
		return nil, fmt.Errorf("failed to new a state database: %w", err)
	}

	// Set the evmCfg config for evmCfg.
	evmCfg := gethvm.Config{
		ExtraEips: []int{EIP3855},
	}

	return &Executor{
		stateDB:    stateDB,
		vmChainCfg: vmChainCfg,
		evmCfg:     evmCfg,
	}, nil
}

// StateDB returns the state DB in the executor.
func (e *Executor) StateDB() *state.StateDB {
	return e.stateDB
}

// DeployContract deploys a contract on the state database in the executor.
// It calls the evm.Create in geth. If the given leftOverGas is not enough, the contract will not be created.
func (e *Executor) DeployContract(ctx gethvm.BlockContext, tx transaction.Transaction) (common.Address, uint64, error) {
	if tx.TxType() != transaction.CreateContractTxType {
		return common.Address{}, 0, fmt.Errorf("not a create contract tx, tx type: %d", tx.TxType())
	}

	txCtx := gethvm.TxContext{
		Origin: common.Address(tx.Sender),
	}

	uValue, err := utils.BigToUInt256(tx.Value)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to convert value to uint256: %w", err)
	}

	evm := gethvm.NewEVM(ctx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)
	// Create(Deploy) a contract.
	_, contractAddress, leftOverGas, err := evm.Create(common.Address(tx.Sender), tx.Data, tx.GasLimit, uValue)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to create contract: %w", err)
	}

	return contractAddress, leftOverGas, nil
}

// CallContract calls the contract with the given `to` address.
// It calls the evm,Call in geth.
func (e *Executor) CallContract(ctx gethvm.BlockContext, tx transaction.Transaction) ([]byte, uint64, error) {
	if tx.TxType() != transaction.CallContractTxType {
		return nil, 0, fmt.Errorf("not a call contract tx, tx type: %d", tx.TxType())
	}

	txCtx := gethvm.TxContext{
		Origin: common.Address(tx.Sender),
	}

	uValue, err := utils.BigToUInt256(tx.Value)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert value to uint256: %w", err)
	}

	evm := gethvm.NewEVM(ctx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)

	return evm.Call(common.Address(tx.Sender), common.Address(tx.Recipient), tx.Data, tx.GasLimit, uValue)
}

// Commit commits the stateDB in the executor.
// Since the executor is committed, it should be aborted or re-created with a new blockCtx.
func (e *Executor) Commit(blockNumber uint64) (common.Hash, error) {
	return e.stateDB.Commit(blockNumber, false, false)
}
