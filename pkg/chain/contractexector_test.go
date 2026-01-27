package chain

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
)

var (
	simpleStorageContractCode = common.Hex2Bytes(
		"608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80636057361d1461003b57806360fe47b11461004f575b600080fd5b61004d6100573660046100c3565b600055565b005b61004d61006b3660046100c3565b600054909101604051918252602082015260400160405180910390f35b60006020828403121561009957600080fd5b5035919050565b600080604083850312156100b257600080fd5b82356001600160a01b03811681146100c957600080fd5b94602093909301359350505056fea26469706673582212202ecdec0bc0105934a0b9904412fa1e46f44e79ba869d59570e515a5e61a4e02a64736f6c634300080f0033",
	)

	// 调用set方法的数据：setValue(1)
	setCallData = common.Hex2Bytes("60fe47b10000000000000000000000000000000000000000000000000000000000000001")
	// 调用get方法的数据
	getCallData = common.Hex2Bytes("6d4ce63c")

	// 测试用的地址
	testSenderAddr = common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")
)

// TestContractExecutorInterface 测试ContractExecutor接口的基本功能
func TestContractExecutorInterface(t *testing.T) {
	// 1. 创建内存数据库
	db := rawdb.NewMemoryDatabase()
	trDB := triedb.NewDatabase(db, nil)
	stateStore := state.NewDatabase(trDB, nil)

	// 2. 创建vm.Executor实例
	vmChainCfg := *params.MainnetChainConfig
	vmChainCfg.ChainID = big.NewInt(1)

	vmChainCfg.HomesteadBlock = big.NewInt(0)
	vmChainCfg.EIP150Block = big.NewInt(0)
	vmChainCfg.EIP155Block = big.NewInt(0)
	vmChainCfg.EIP158Block = big.NewInt(0)
	vmChainCfg.ByzantiumBlock = big.NewInt(0)
	vmChainCfg.ConstantinopleBlock = big.NewInt(0)
	vmChainCfg.PetersburgBlock = big.NewInt(0)
	vmChainCfg.IstanbulBlock = big.NewInt(0)
	vmChainCfg.BerlinBlock = big.NewInt(0)
	vmChainCfg.LondonBlock = big.NewInt(0)
	vmChainCfg.ShanghaiTime = new(uint64)
	*vmChainCfg.ShanghaiTime = 0

	vmExec, err := vm.NewExecutor(stateStore, types.EmptyRootHash, &vmChainCfg)
	require.NoError(t, err)

	// 3. 设置测试账户的初始余额
	vmExec.StateDB().AddBalance(testSenderAddr, uint256.NewInt(1000000000), tracing.BalanceChangeTransfer)

	// 4. 创建测试用的BlockContext
	blockCtx := gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		BlockNumber: big.NewInt(1),
		Time:        uint64(time.Now().Unix()),
		GasLimit:    1000000,
		BaseFee:     big.NewInt(1),
	}

	// 5. 创建ContractTx实例（实现了ContractExecutor接口）
	contractTx := &ContractTx{
		vmChainCfg: &vmChainCfg,
	}

	// 6. 验证ContractTx实现了ContractExecutor接口
	var executor ContractExecutor = contractTx
	require.NotNil(t, executor)

	// 7. 测试CreateContractTxExecute方法（部署合约）
	t.Run("CreateContractTxExecute", func(t *testing.T) {
		createTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.EmptyAccountAddr, // 空地址表示创建合约
			Data:       simpleStorageContractCode,
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		contractAddr, leftOverGas, err := contractTx.CreateContractTxExecute(vmExec, blockCtx, createTx)
		require.NoError(t, err)
		require.NotZero(t, contractAddr)
		require.Greater(t, leftOverGas, uint64(0))

		// 验证合约已被创建
		code := vmExec.StateDB().GetCode(contractAddr)

		fmt.Println(len(code))
		fmt.Printf("%x\n", code[:20])
		require.NotEmpty(t, code)
	})

	// 8. 测试CallContractTxExecute方法（调用合约）
	t.Run("CallContractTxExecute", func(t *testing.T) {
		// 先部署合约获取地址
		createTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.EmptyAccountAddr,
			Data:       simpleStorageContractCode,
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		contractTx := &ContractTx{
			vmChainCfg: &vmChainCfg,
		}

		contractAddr, _, err := contractTx.CreateContractTxExecute(vmExec, blockCtx, createTx)
		require.NoError(t, err)

		// 测试调用set方法
		setTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.Address(contractAddr),
			Data:       setCallData,
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		_, leftOverGas, err := contractTx.CallContractTxExecute(vmExec, blockCtx, setTx)
		require.NoError(t, err)
		require.Greater(t, leftOverGas, uint64(0))

		// 测试调用get方法
		getTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.Address(contractAddr),
			Data:       getCallData,
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		ret, leftOverGas, err := contractTx.CallContractTxExecute(vmExec, blockCtx, getTx)
		require.NoError(t, err)
		require.Greater(t, leftOverGas, uint64(0))

		// 验证返回值是否为1（从大端字节序转换）
		resultValue := new(big.Int).SetBytes(ret)
		expectedValue := big.NewInt(1)
		require.Equal(t, expectedValue, resultValue, "Expected returned value to be 1")
	})

	// 9. 测试错误情况：使用错误的交易类型
	t.Run("ErrorHandling", func(t *testing.T) {
		// 创建一个新的ContractTx实例
		contractTx := &ContractTx{
			vmChainCfg: &vmChainCfg,
		}

		// 创建一个合约以获得地址
		createTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.EmptyAccountAddr,
			Data:       simpleStorageContractCode,
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		contractAddr, _, err := contractTx.CreateContractTxExecute(vmExec, blockCtx, createTx)
		require.NoError(t, err)

		// 测试用普通交易调用CreateContractTxExecute
		normalTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.Address(contractAddr),
			Value:      big.NewInt(100),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		_, _, err = contractTx.CreateContractTxExecute(vmExec, blockCtx, normalTx)
		require.Error(t, err) // 应该返回错误，因为不是创建合约的交易类型

		// 测试用普通交易调用CallContractTxExecute
		_, _, err = contractTx.CallContractTxExecute(vmExec, blockCtx, normalTx)
		require.Error(t, err) // 应该返回错误，因为不是调用合约的交易类型
	})
}

// TestContractExecutorIntegration 测试ContractExecutor接口的集成功能
func TestContractExecutorIntegration(t *testing.T) {
	// 1. 创建内存数据库
	db := rawdb.NewMemoryDatabase()
	trDB := triedb.NewDatabase(db, nil)
	stateStore := state.NewDatabase(trDB, nil)

	// 2. 创建vm.Executor实例
	vmChainCfg := *params.MainnetChainConfig
	vmChainCfg.ChainID = big.NewInt(1)

	vmChainCfg.HomesteadBlock = big.NewInt(0)
	vmChainCfg.EIP150Block = big.NewInt(0)
	vmChainCfg.EIP155Block = big.NewInt(0)
	vmChainCfg.EIP158Block = big.NewInt(0)
	vmChainCfg.ByzantiumBlock = big.NewInt(0)
	vmChainCfg.ConstantinopleBlock = big.NewInt(0)
	vmChainCfg.PetersburgBlock = big.NewInt(0)
	vmChainCfg.IstanbulBlock = big.NewInt(0)
	vmChainCfg.BerlinBlock = big.NewInt(0)
	vmChainCfg.LondonBlock = big.NewInt(0)
	vmChainCfg.ShanghaiTime = new(uint64)
	*vmChainCfg.ShanghaiTime = 0

	vmExec, err := vm.NewExecutor(stateStore, types.EmptyRootHash, &vmChainCfg)
	require.NoError(t, err)

	// 3. 设置测试账户的初始余额
	vmExec.StateDB().AddBalance(testSenderAddr, uint256.NewInt(1000000000), tracing.BalanceChangeTransfer)

	// 4. 创建测试用的BlockContext
	blockCtx := gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		BlockNumber: big.NewInt(1),
		Time:        uint64(time.Now().Unix()),
		GasLimit:    1000000,
		BaseFee:     big.NewInt(1),
	}

	// 5. 创建ContractTx实例
	contractTx := &ContractTx{
		vmChainCfg: &vmChainCfg,
	}

	// 6. 部署合约
	createTx := transaction.Transaction{
		Sender:     account.Address(testSenderAddr),
		Recipient:  account.EmptyAccountAddr,
		Data:       simpleStorageContractCode,
		Value:      big.NewInt(0),
		GasLimit:   1000000,
		CreateTime: time.Now(),
	}

	contractAddr, _, err := contractTx.CreateContractTxExecute(vmExec, blockCtx, createTx)
	require.NoError(t, err)
	require.NotZero(t, contractAddr)

	// 7. 连续调用合约多次，测试状态是否正确维护
	for i := 1; i <= 5; i++ {
		// 构造设置特定值的数据
		setData := append(common.Hex2Bytes("60fe47b1"), common.LeftPadBytes(big.NewInt(int64(i)).Bytes(), 32)...)

		// 调用set方法设置值
		setTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.Address(contractAddr),
			Data:       setData, // 设置值为i
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		_, _, err := contractTx.CallContractTxExecute(vmExec, blockCtx, setTx)
		require.NoError(t, err)

		// 调用get方法获取值
		getTx := transaction.Transaction{
			Sender:     account.Address(testSenderAddr),
			Recipient:  account.Address(contractAddr),
			Data:       getCallData,
			Value:      big.NewInt(0),
			GasLimit:   1000000,
			CreateTime: time.Now(),
		}

		ret, _, err := contractTx.CallContractTxExecute(vmExec, blockCtx, getTx)
		require.NoError(t, err)

		// 验证返回值是否为当前循环的i值
		resultValue := new(big.Int).SetBytes(ret)
		expectedValue := big.NewInt(int64(i))
		require.Equal(t, expectedValue, resultValue, "Expected returned value to be %d", i)
	}
}

// TestContractTxStructDirectly 测试ContractTx结构体的直接实现
func TestContractTxStructDirectly(t *testing.T) {
	// 1. 创建内存数据库
	db := rawdb.NewMemoryDatabase()
	trDB := triedb.NewDatabase(db, nil)
	stateStore := state.NewDatabase(trDB, nil)

	// 2. 创建vm.Executor实例
	vmChainCfg := *params.MainnetChainConfig
	vmChainCfg.ChainID = big.NewInt(1)

	vmChainCfg.HomesteadBlock = big.NewInt(0)
	vmChainCfg.EIP150Block = big.NewInt(0)
	vmChainCfg.EIP155Block = big.NewInt(0)
	vmChainCfg.EIP158Block = big.NewInt(0)
	vmChainCfg.ByzantiumBlock = big.NewInt(0)
	vmChainCfg.ConstantinopleBlock = big.NewInt(0)
	vmChainCfg.PetersburgBlock = big.NewInt(0)
	vmChainCfg.IstanbulBlock = big.NewInt(0)
	vmChainCfg.BerlinBlock = big.NewInt(0)
	vmChainCfg.LondonBlock = big.NewInt(0)
	vmChainCfg.ShanghaiTime = new(uint64)
	*vmChainCfg.ShanghaiTime = 0

	vmExec, err := vm.NewExecutor(stateStore, types.EmptyRootHash, &vmChainCfg)
	require.NoError(t, err)

	// 3. 设置测试账户的初始余额
	vmExec.StateDB().AddBalance(testSenderAddr, uint256.NewInt(1000000000), tracing.BalanceChangeTransfer)

	// 4. 创建测试用的BlockContext
	blockCtx := gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		BlockNumber: big.NewInt(1),
		Time:        uint64(time.Now().Unix()),
		GasLimit:    1000000,
		BaseFee:     big.NewInt(1),
	}

	// 5. 创建ContractTx实例
	contractTx := &ContractTx{
		vmChainCfg: &vmChainCfg,
	}

	// 6. 部署合约
	createTx := transaction.Transaction{
		Sender:     account.Address(testSenderAddr),
		Recipient:  account.EmptyAccountAddr,
		Data:       simpleStorageContractCode,
		Value:      big.NewInt(0),
		GasLimit:   1000000,
		CreateTime: time.Now(),
	}

	contractAddr, _, err := contractTx.CreateContractTxExecute(vmExec, blockCtx, createTx)
	require.NoError(t, err)
	require.NotZero(t, contractAddr)

	// 7. 调用合约方法
	setTx := transaction.Transaction{
		Sender:     account.Address(testSenderAddr),
		Recipient:  account.Address(contractAddr),
		Data:       setCallData,
		Value:      big.NewInt(0),
		GasLimit:   1000000,
		CreateTime: time.Now(),
	}

	_, _, err = contractTx.CallContractTxExecute(vmExec, blockCtx, setTx)
	require.NoError(t, err)

	// 8. 获取值验证
	getTx := transaction.Transaction{
		Sender:     account.Address(testSenderAddr),
		Recipient:  account.Address(contractAddr),
		Data:       getCallData,
		Value:      big.NewInt(0),
		GasLimit:   1000000,
		CreateTime: time.Now(),
	}

	ret, _, err := contractTx.CallContractTxExecute(vmExec, blockCtx, getTx)
	require.NoError(t, err)

	// 验证返回值
	resultValue := new(big.Int).SetBytes(ret)
	expectedValue := big.NewInt(1)
	require.Equal(t, expectedValue, resultValue, "Expected returned value to be 1")
}
