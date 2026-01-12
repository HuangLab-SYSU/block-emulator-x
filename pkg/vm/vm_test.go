package vm

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
)

func TestVM(t *testing.T) {
	// 从JSON-RPC请求中提取的数据
	from := common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")
	gas := uint64(0x816500) // 530,000
	value := uint256.NewInt(0)
	data := common.Hex2Bytes("6080604052348015600e575f5ffd5b506101298061001c5f395ff3fe6080604052348015600e575f5ffd5b50600436106030575f3560e01c806360fe47b11460345780636d4ce63c14604c575b5f5ffd5b604a60048036038101906046919060a9565b6066565b005b6052606f565b604051605d919060dc565b60405180910390f35b805f8190555050565b5f5f54905090565b5f5ffd5b5f819050919050565b608b81607b565b81146094575f5ffd5b50565b5f8135905060a3816084565b92915050565b5f6020828403121560bb5760ba6077565b5b5f60c6848285016097565b91505092915050565b60d681607b565b82525050565b5f60208201905060ed5f83018460cf565b9291505056fea264697066735822122018e99961d9a131ff1e37f753c49a557446ca61080d46660ca34f7d6065d567c364736f6c634300081f0033")

	// 创建状态数据库

	level, err := leveldb.New("testdir", 0, 0, "eee", false)
	if err != nil {
		panic(err)
	}

	db := rawdb.NewDatabase(level)

	trieDb := triedb.NewDatabase(db, &triedb.Config{
		Preimages: true,
		IsVerkle:  false,
	})

	sp, err := snapshot.New(snapshot.Config{CacheSize: 1 << 5}, db, trieDb, types.EmptyRootHash)
	if err != nil {
		panic(err)
	}
	cachedb := state.NewDatabase(trieDb, sp)

	statedb, err := state.New(types.EmptyRootHash, cachedb)

	if err != nil {
		fmt.Printf("创建状态数据库失败: %v\n", err)
		return
	}

	// 为发送者账户添加初始余额 (1 ETH)
	initialBalance := uint256.NewInt(100000)
	statedb.AddBalance(from, initialBalance, tracing.BalanceChangeTransfer)

	// 配置EVM环境
	//chainConfig := &params.ChainConfig{
	// ChainID: common.Big1,
	//}

	chainConfig := params.MainnetChainConfig

	blockContext := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		//BlockNumber: common.Big1,
		BlockNumber: big.NewInt(20_000_000),
		//Time:        0,
		Time:       1700000000,
		Difficulty: common.Big0,
		GasLimit:   gas,
		BaseFee:    big.NewInt(1),
	}

	txContext := vm.TxContext{
		Origin:   from,
		GasPrice: big.NewInt(100),
	}

	//evmConfig := vm.Config{}
	evmConfig := vm.Config{
		ExtraEips: []int{3855},
	}

	fmt.Println("Block time:", blockContext.Time)
	fmt.Println("Shanghai time:", *params.MainnetChainConfig.ShanghaiTime)
	fmt.Println("IsShanghai:",
		params.MainnetChainConfig.IsShanghai(
			blockContext.BlockNumber,
			blockContext.Time,
		),
	)

	// 部署合约
	fmt.Println("部署智能合约...")
	contractAddress, contractCode, _, err := deployContract(statedb, blockContext, txContext, chainConfig, evmConfig, from, data, value, gas)
	if err != nil {
		fmt.Printf("合约部署失败: %v\n", err)
		return
	}

	fmt.Printf("合约部署成功! 地址: 0x%x\n", contractAddress)
	fmt.Printf("合约代码长度: %d 字节\n", len(contractCode))

	// 调用合约方法 (调用合约的balanceOf方法示例)
	fmt.Println("\n调用合约方法...")
	// 构造balanceOf方法调用数据: 方法签名(balanceOf(address)) + 参数(from地址)
	balanceOfSig := common.Hex2Bytes("60fe47b10000000000000000000000000000000000000000000000000000000000000001") // balanceOf(address)的方法签名

	fmt.Println(balanceOfSig)
	callResult, err := callContract(statedb, blockContext, txContext, chainConfig, evmConfig, from, contractAddress, balanceOfSig, value, gas)
	if err != nil {
		fmt.Printf("合约调用失败: %v\n", err)
		return
	}

	// 解析结果 (balanceOf返回的是uint256值)
	if len(callResult) == 32 {
		resultInt := new(uint256.Int).SetBytes(callResult)
		fmt.Printf("合约调用结果(balanceOf): %s\n", resultInt.String())
	} else {
		fmt.Printf("合约调用结果: 0x%x\n", callResult)
	}

	getData := common.Hex2Bytes("6d4ce63c")
	callGetResult, err := callContract(statedb, blockContext, txContext, chainConfig, evmConfig, from, contractAddress, getData, value, gas)
	if err != nil {
		fmt.Printf("合约调用失败: %v\n", err)
		return
	}

	// 解析结果 (balanceOf返回的是uint256值)
	if len(callGetResult) == 32 {
		resultInt := new(uint256.Int).SetBytes(callGetResult)
		fmt.Printf("合约调用结果(balanceOf): %s\n", resultInt.String())
	} else {
		fmt.Printf("合约调用结果: 0x%x\n", callGetResult)
	}

	_, err = statedb.Commit(20_000_000, true, true)
	if err != nil {
		panic(err)
	}
	fmt.Println(statedb.GetBalance(from))
}
