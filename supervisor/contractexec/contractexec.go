package contractexec

import (
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/storage/vmstate"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

const (
	blockNumberBias = 20_000_000
	blockGasLimit   = 1000000
	txCommitBatch   = 200
)

type ContractExec struct {
	stateStore state.Database
	root       common.Hash
	vmChainCfg *params.ChainConfig

	curVMExec   *vm.Executor
	curBlockCtx *gethvm.BlockContext
	txCommitCnt int64

	mux sync.Mutex
}

func NewContractExec(cfg config.Config, lp config.LocalParams) (*ContractExec, error) {
	stateStore, err := vmstate.NewStateStore(cfg.StorageCfg, lp)
	if err != nil {
		return nil, fmt.Errorf("new state store failed: %w", err)
	}

	cc := *params.MainnetChainConfig
	cc.ChainID = big.NewInt(cfg.ChainID)

	bCtx := &gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		GasLimit:    blockGasLimit,
		BlockNumber: big.NewInt(blockNumberBias),
		Time:        uint64(time.Now().Unix()),
	}

	e, err := vm.NewExecutor(stateStore, types.EmptyRootHash, &cc)
	if err != nil {
		return nil, fmt.Errorf("new an vm executor failed: %w", err)
	}

	return &ContractExec{
		stateStore: stateStore,
		root:       types.EmptyRootHash,
		vmChainCfg: &cc,

		curVMExec:   e,
		curBlockCtx: bCtx,
		txCommitCnt: 0,
	}, nil
}

func (c *ContractExec) ContractTxExec(tx transaction.Transaction) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	switch tx.TxType() {
	case transaction.CreateContractTxType:
		contractAddr, leftOverGas, err := c.curVMExec.DeployContract(*c.curBlockCtx, tx)
		if err != nil {
			return fmt.Errorf("deploy contract in vm executor failed: %w", err)
		}

		slog.Info("deploy contract succeed", "contractAddr", contractAddr, "leftOverGas", leftOverGas)

	case transaction.CallContractTxType:
		ret, gasLeft, err := c.curVMExec.CallContract(*c.curBlockCtx, tx)
		if err != nil {
			return fmt.Errorf("call contract in vm executor failed: %w", err)
		}

		slog.Info("call tx succeed", "result len", len(ret), "gasLeft", gasLeft)

	default:
		return fmt.Errorf("not a contract tx, tx type: %d", tx.TxType())
	}

	if err := c.tryResetVMExecAndBlockCtx(); err != nil {
		return fmt.Errorf("reset vm executor and block context failed: %w", err)
	}

	return nil
}

func (c *ContractExec) tryResetVMExecAndBlockCtx() error {
	c.txCommitCnt++
	if c.txCommitCnt < txCommitBatch {
		return nil
	}
	// Reset the txCommitCnt to 0.
	c.txCommitCnt = 0

	oldBCtx := c.curBlockCtx
	// Reset VM executor - 1. Commit the old vm state DB.
	root, err := c.curVMExec.Commit(oldBCtx.BlockNumber.Uint64())
	if err != nil {
		return fmt.Errorf("vm executor commit failed: %w", err)
	}

	if err = c.stateStore.TrieDB().Commit(root, false); err != nil {
		return fmt.Errorf("trie db commit failed: %w", err)
	}
	// Reset VM executor - 2. Renew a vm state DB.
	c.curVMExec, err = vm.NewExecutor(c.stateStore, root, c.vmChainCfg)
	if err != nil {
		return fmt.Errorf("new vm executor failed: %w", err)
	}

	// Reset block context.
	newCtx := &gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		GasLimit:    blockGasLimit,
		BlockNumber: new(big.Int).Add(oldBCtx.BlockNumber, big.NewInt(1)),
		Time:        uint64(time.Now().Unix()),
	}
	c.curBlockCtx = newCtx

	return nil
}
