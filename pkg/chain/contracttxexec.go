package chain

import (
	"github.com/ethereum/go-ethereum/common"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

type ContractTx struct {
	vmChainCfg *params.ChainConfig
}

func (ctx *ContractTx) CreateContractTxExecute(
	v *vm.Executor,
	bCtx gethvm.BlockContext,
	tx transaction.Transaction,
) (common.Address, uint64, error) {
	contractAddr, leftOverGas, err := v.DeployContract(bCtx, tx)
	return contractAddr, leftOverGas, err
}

func (ctx *ContractTx) CallContractTxExecute(
	v *vm.Executor,
	bCtx gethvm.BlockContext,
	tx transaction.Transaction,
) ([]byte, uint64, error) {
	ret, leftOverGas, err := v.CallContract(bCtx, tx)
	return ret, leftOverGas, err
}
