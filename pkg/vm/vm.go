package vm

import (
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	vmDBNamespace       = "vm_trie"
	defaultLevelCache   = 16
	defaultLevelHandler = 16
	vmStateDBPathFmt    = "vm_state_shard_%d_node_%d"
	snapshotCacheMB     = 1 << 5
	EIP3855             = 3855
)

type Executor struct {
	stateDB    *state.StateDB
	vmChainCfg *params.ChainConfig
	evmCfg     gethvm.Config
}

func NewExecutor(cfg config.VMCfg, lp config.LocalParams) (*Executor, error) {
	// Init state db.
	dbPath := filepath.Join(cfg.VMStateDir, fmt.Sprintf(vmStateDBPathFmt, lp.ShardID, lp.NodeID))

	level, err := leveldb.New(dbPath, defaultLevelCache, defaultLevelHandler, vmDBNamespace, false)
	if err != nil {
		return nil, fmt.Errorf("failed to new a level database: %w", err)
	}

	db := rawdb.NewDatabase(level)
	trDB := triedb.NewDatabase(db, &triedb.Config{Preimages: true, IsVerkle: false})
	dbRoot := types.EmptyRootHash

	sp, err := snapshot.New(snapshot.Config{CacheSize: snapshotCacheMB}, db, trDB, dbRoot)
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
		stateDB:    stateDB,
		vmChainCfg: &cc,
		evmCfg:     evmCfg,
	}, nil
}

func (e *Executor) StateDB() *state.StateDB {
	return e.stateDB
}

func (e *Executor) DeployContract(
	blockCtx gethvm.BlockContext,
	txCtx gethvm.TxContext,
	from common.Address,
	code []byte,
	value *uint256.Int,
	gasLimit uint64,
) (common.Address, uint64, error) {
	evm := gethvm.NewEVM(blockCtx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)
	// Create(Deploy) a contract.
	_, contractAddress, gasUsed, err := evm.Create(from, code, gasLimit, value)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("failed to create contract: %w", err)
	}

	return contractAddress, gasUsed, nil
}

func (e *Executor) CallContract(
	blockCtx gethvm.BlockContext,
	txCtx gethvm.TxContext,
	from, to common.Address,
	data []byte,
	value *uint256.Int,
	gasLimit uint64,
) ([]byte, uint64, error) {
	evm := gethvm.NewEVM(blockCtx, e.stateDB, e.vmChainCfg, e.evmCfg)
	evm.SetTxContext(txCtx)

	return evm.Call(from, to, data, gasLimit, value)
}
