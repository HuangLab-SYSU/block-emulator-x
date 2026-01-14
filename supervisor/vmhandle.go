package supervisor

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const vmStateNameSpace = "vm_state"

type VMHandle struct {
	trDB *triedb.Database
	root common.Hash
}

func NewVMHandle(cfg config.VMCfg) (*VMHandle, error) {
	level, err := leveldb.New(cfg.VMStateDir, 0, 0, vmStateNameSpace, false)
	if err != nil {
		return nil, err
	}

	return &VMHandle{
		trDB: triedb.NewDatabase(rawdb.NewDatabase(level), &triedb.Config{Preimages: true, IsVerkle: false}),
		root: types.EmptyRootHash,
	}, nil
}

func (v *VMHandle) handleTx() error {
	panic("implement me")
}
