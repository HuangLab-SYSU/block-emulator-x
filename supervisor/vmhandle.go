package supervisor

import (
	"fmt"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

const (
	vmStateNameSpace = "vm_state"
	blockGasLimit    = uint64(1000000)
	unlimitedGas     = uint64(1000000)
)

type VMHandle struct {
	trDB       *triedb.Database
	root       common.Hash
	vmChainCfg *params.ChainConfig
}

func NewVMHandle(cfg config.VMCfg) (*VMHandle, error) {
	level, err := leveldb.New(cfg.VMStateDir, 0, 0, vmStateNameSpace, false)
	if err != nil {
		return nil, err
	}

	cc := *params.MainnetChainConfig
	cc.ChainID = big.NewInt(cfg.ChainID)

	return &VMHandle{
		trDB:       triedb.NewDatabase(rawdb.NewDatabase(level), &triedb.Config{Preimages: true, IsVerkle: false}),
		root:       types.EmptyRootHash,
		vmChainCfg: &cc,
	}, nil
}

func (v *VMHandle) HandleBlock(b block.Block) error {
	bCtx := gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		GasLimit:    blockGasLimit,
		BlockNumber: new(big.Int).SetUint64(b.Number),
		Time:        uint64(b.CreateTime.Second()),
	}

	e, err := vm.NewExecutor(bCtx, v.trDB, v.root, v.vmChainCfg)
	if err != nil {
		return fmt.Errorf("new an vm executor failed: %w", err)
	}

	// Handle transactions
	for _, tx := range b.TxList {
		txCtx := gethvm.TxContext{
			Origin: common.Address(tx.Sender),
		}
		switch tx.TxType() {
		case transaction.CreateContractTxType:
			contractAddr, gasUsed, err := e.DeployContract(txCtx, tx.Sender, tx.Data, tx.Value, unlimitedGas)
			if err != nil {
				slog.Error("deploy contract tx failed", "err", err)
			} else {
				slog.Info("deploy contract succeed", "contractAddr", contractAddr, "gasUsed", gasUsed)
			}
		case transaction.CallContractTxType:
			ret, gasLeft, err := e.CallContract(txCtx, tx.Sender, tx.Recipient, tx.Data, tx.Value, unlimitedGas)
			if err != nil {
				slog.Error("deploy contract tx failed", "err", err)
			} else {
				slog.Info("call tx succeed", "result", ret, "gasLeft", gasLeft)
			}
		default:
			slog.Debug("not a transaction about contract, skip it")
		}
	}

	root, err := e.Commit()
	if err != nil {
		return fmt.Errorf("commit state database in vm failed: %w", err)
	}

	slog.Info("handle block succeed", "new state root", root)

	return nil
}
