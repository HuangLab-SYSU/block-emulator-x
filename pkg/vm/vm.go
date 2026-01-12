package vm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	snapshotCacheMB = 1 << 5
	EIP3855         = 3855
)

type Executor struct {
	bCtx       gethvm.BlockContext
	stateDB    *state.StateDB
	vmChainCfg *params.ChainConfig
	evmCfg     gethvm.Config
}

func NewExecutor(
	bCtx gethvm.BlockContext,
	edb ethdb.Database,
	curRoot [32]byte,
	cfg config.VMCfg,
) (*Executor, error) {
	// Init state db.
	trDB := triedb.NewDatabase(edb, &triedb.Config{Preimages: true, IsVerkle: false})
	dbRoot := curRoot

	sp, err := snapshot.New(snapshot.Config{CacheSize: snapshotCacheMB}, edb, trDB, dbRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to new a snapshot: %w", err)
	}

	stateDB, err := state.New(dbRoot, state.NewDatabase(trDB, sp))
	if err != nil {
		return nil, fmt.Errorf("failed to new a state database: %w", err)
	}

	// Set the chainID to the chainConfig.
	cc := *params.MainnetChainConfig
	cc.ChainID = big.NewInt(cfg.ChainID)

	// Set the evmCfg config for evmCfg.
	evmCfg := gethvm.Config{
		ExtraEips: []int{EIP3855},
	}

	return &Executor{
		bCtx:       bCtx,
		stateDB:    stateDB,
		vmChainCfg: &cc,
		evmCfg:     evmCfg,
	}, nil
}

func (e *Executor) StateDB() *state.StateDB {
	return e.stateDB
}

func (e *Executor) DeployContract(
	txCtx gethvm.TxContext,
	from common.Address,
	code []byte,
	value *uint256.Int,
	gasLimit uint64,
) (common.Address, uint64, error) {
	evm := gethvm.NewEVM(e.bCtx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)
	// Create(Deploy) a contract.
	_, contractAddress, gasUsed, err := evm.Create(from, code, gasLimit, value)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to create contract: %w", err)
	}

	return contractAddress, gasUsed, nil
}

func (e *Executor) CallContract(
	txCtx gethvm.TxContext,
	from, to common.Address,
	data []byte,
	value *uint256.Int,
	gasLimit uint64,
) ([]byte, uint64, error) {
	evm := gethvm.NewEVM(e.bCtx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)

	return evm.Call(from, to, data, gasLimit, value)
}

func (e *Executor) Commit() ([]byte, error) {
	root, err := e.stateDB.Commit(e.bCtx.BlockNumber.Uint64(), true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to commit state: %w", err)
	}

	return root[:], nil
}
