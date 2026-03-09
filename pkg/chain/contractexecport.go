package chain

import (
	"github.com/ethereum/go-ethereum/common"
	gethvm "github.com/ethereum/go-ethereum/core/vm"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

type ContractExecPort interface {
	CreateContractTxExecute(
		v *vm.Executor,
		bCtx gethvm.BlockContext,
		tx transaction.Transaction,
	) (common.Address, uint64, error)
	CallContractTxExecute(v *vm.Executor, bCtx gethvm.BlockContext, tx transaction.Transaction) ([]byte, uint64, error)
}
