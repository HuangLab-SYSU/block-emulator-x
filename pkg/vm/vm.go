package vm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
)

const (
	snapshotCacheMB = 1 << 5
	EIP3855         = 3855
)

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
func NewExecutor(trDB *triedb.Database, root common.Hash, vmChainCfg *params.ChainConfig) (*Executor, error) {
	// Init state db.
	sp, err := snapshot.New(snapshot.Config{CacheSize: snapshotCacheMB}, trDB.Disk(), trDB, root)
	if err != nil {
		return nil, fmt.Errorf("failed to new a snapshot: %w", err)
	}

	stateDB, err := state.New(root, state.NewDatabase(trDB, sp))
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
func (e *Executor) DeployContract(
	bCtx gethvm.BlockContext,
	txCtx gethvm.TxContext,
	from account.Address,
	code []byte,
	value *big.Int,
	leftOverGas uint64,
) (common.Address, uint64, error) {
	uValue, err := utils.BigToUInt256(value)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to convert value to uint256: %w", err)
	}

	evm := gethvm.NewEVM(bCtx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)
	// Create(Deploy) a contract.
	_, contractAddress, gasUsed, err := evm.Create(common.Address(from), code, leftOverGas, uValue)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to create contract: %w", err)
	}

	return contractAddress, gasUsed, nil
}

// CallContract calls the contract with the given `to` address.
// It calls the evm,Call in geth.
func (e *Executor) CallContract(
	bCtx gethvm.BlockContext,
	txCtx gethvm.TxContext,
	from, to account.Address,
	data []byte,
	value *big.Int,
	leftOverGas uint64,
) ([]byte, uint64, error) {
	uValue, err := utils.BigToUInt256(value)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert value to uint256: %w", err)
	}

	evm := gethvm.NewEVM(bCtx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)

	return evm.Call(common.Address(from), common.Address(to), data, leftOverGas, uValue)
}

// Commit commits the stateDB in the executor.
// Since the executor is committed, it should be aborted or re-created with a new blockCtx.
func (e *Executor) Commit(blockNumber uint64) (common.Hash, error) {
	return e.stateDB.Commit(blockNumber, true, false)
}
