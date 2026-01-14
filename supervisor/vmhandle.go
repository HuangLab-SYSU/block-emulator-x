package supervisor

import (
	"fmt"
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
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

const vmStateNameSpace = "vm_state"

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
		GasLimit:    1000000,
		BlockNumber: new(big.Int).SetUint64(b.Number),
		Time:        uint64(b.CreateTime.Second()),
	}

	_, err := vm.NewExecutor(bCtx, v.trDB, v.root, v.vmChainCfg)
	if err != nil {
		return fmt.Errorf("new an vm executor failed: %w", err)
	}

	panic("implement me")
}
