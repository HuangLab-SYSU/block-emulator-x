package vm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

type Executor struct {
}

func NewExecutor() *Executor {

	return &Executor{}
}

func (e *Executor) Deploy() error {
	panic("implement me")
}

func (e *Executor) Call() ([]byte, error) {
	panic("implement me")
}

// deployContract 部署智能合约
func deployContract(statedb *state.StateDB, blockContext gethvm.BlockContext, txContext gethvm.TxContext,
	chainConfig *params.ChainConfig, evmConfig gethvm.Config, from common.Address,
	code []byte, value *uint256.Int, gasLimit uint64) (common.Address, []byte, uint64, error) {

	evm := gethvm.NewEVM(blockContext, statedb, chainConfig, evmConfig)
	evm.SetTxContext(txContext)
	// 部署合约 (CREATE操作)
	_, contractAddress, gasUsed, err := evm.Create(from, code, gasLimit, value)
	if err != nil {
		return common.Address{}, nil, 0, err
	}

	// 获取部署后的合约代码
	contractCode := statedb.GetCode(contractAddress)
	return contractAddress, contractCode, gasUsed, nil
}

// callContract 调用智能合约方法
func callContract(statedb *state.StateDB, blockContext gethvm.BlockContext, txContext gethvm.TxContext,
	chainConfig *params.ChainConfig, evmConfig gethvm.Config, from common.Address,
	to common.Address, data []byte, value *uint256.Int, gasLimit uint64) ([]byte, error) {

	evm := gethvm.NewEVM(blockContext, statedb, chainConfig, evmConfig)
	evm.SetTxContext(txContext)
	// 调用合约方法
	result, _, err := evm.Call(from, to, data, gasLimit, value)
	return result, err
}
