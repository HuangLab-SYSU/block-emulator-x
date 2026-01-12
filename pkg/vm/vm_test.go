package vm

import (
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
)

const (
	testVMStateDir = "test_vm_state"
)

var (
	blockCtx = vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		BlockNumber: big.NewInt(20_000_000),
		Time:        1700000000,
		Difficulty:  common.Big0,
		GasLimit:    uint64(0x816500),
		BaseFee:     big.NewInt(1),
	}
	from           = common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")
	initialBalance = uint256.NewInt(1000000000)
	gasLimit       = uint64(530000)
	txVal          = uint256.NewInt(0)
	contractCode   = common.Hex2Bytes(
		"6080604052348015600e575f5ffd5b506101298061001c5f395ff3fe6080604052348015600e575f5ffd5" +
			"b50600436106030575f3560e01c806360fe47b11460345780636d4ce63c14604c575b5f5ffd5b604a6004" +
			"8036038101906046919060a9565b6066565b005b6052606f565b604051605d919060dc565b60405180910" +
			"390f35b805f8190555050565b5f5f54905090565b5f5ffd5b5f819050919050565b608b81607b565b8114" +
			"6094575f5ffd5b50565b5f8135905060a3816084565b92915050565b5f6020828403121560bb5760ba607" +
			"7565b5b5f60c6848285016097565b91505092915050565b60d681607b565b82525050565b5f6020820190" +
			"5060ed5f83018460cf565b9291505056fea264697066735822122018e99961d9a131ff1e37f753c49a557" +
			"446ca61080d46660ca34f7d6065d567c364736f6c634300081f0033",
	)

	setCallData = common.Hex2Bytes(
		"60fe47b10000000000000000000000000000000000000000000000000000000000000001",
	)
	getCallData = common.Hex2Bytes("6d4ce63c")
)

func TestVM(t *testing.T) {
	_ = os.RemoveAll(testVMStateDir)
	t.Cleanup(func() { _ = os.RemoveAll(testVMStateDir) })

	vmExec, err := NewExecutor(config.VMCfg{VMStateDir: testVMStateDir}, config.LocalParams{})
	require.NoError(t, err)

	vmExec.StateDB().AddBalance(from, initialBalance, tracing.BalanceChangeTransfer)

	txContext := vm.TxContext{
		Origin:   from,
		GasPrice: big.NewInt(100),
	}

	// Deploy the contract.
	contractAddr, _, err := vmExec.DeployContract(blockCtx, txContext, from, contractCode, txVal, gasLimit)
	require.NoError(t, err)

	// Call `set` (1).
	callResult, _, err := vmExec.CallContract(blockCtx, txContext, from, contractAddr, setCallData, txVal, gasLimit)
	require.NoError(t, err)
	require.Len(t, callResult, 0)

	// Call `get`.
	callResult, _, err = vmExec.CallContract(blockCtx, txContext, from, contractAddr, getCallData, txVal, gasLimit)
	require.NoError(t, err)

	resultInt := new(uint256.Int).SetBytes(callResult).Uint64()
	require.Equal(t, resultInt, uint64(1))
}
