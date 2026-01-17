package contractexec

import (
	"math/big"
	"testing"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var (
	contractCode = common.Hex2Bytes(
		"6080604052348015600e575f5ffd5b506101298061001c5f395ff3fe6080604052348015600e575f5ffd5" +
			"b50600436106030575f3560e01c806360fe47b11460345780636d4ce63c14604c575b5f5ffd5b604a6004" +
			"8036038101906046919060a9565b6066565b005b6052606f565b604051605d919060dc565b60405180910" +
			"390f35b805f8190555050565b5f5f54905090565b5f5ffd5b5f819050919050565b608b81607b565b8114" +
			"6094575f5ffd5b50565b5f8135905060a3816084565b92915050565b5f6020828403121560bb5760ba607" +
			"7565b5b5f60c6848285016097565b91505092915050565b60d681607b565b82525050565b5f6020820190" +
			"5060ed5f83018460cf565b9291505056fea264697066735822122018e99961d9a131ff1e37f753c49a557" +
			"446ca61080d46660ca34f7d6065d567c364736f6c634300081f0033",
	)

	// call: set(1)
	setCallData = common.Hex2Bytes(
		"60fe47b10000000000000000000000000000000000000000000000000000000000000001",
	)
	// call: get
	getCallData = common.Hex2Bytes("6d4ce63c")

	from         = common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")
	contractAddr = common.HexToAddress("0x87e9100fe2b300c290cf0079a058c3450fd86752")
)

func TestContractExec(t *testing.T) {
	cfg := config.Config{}
	cfg.IsMemoryDB = true

	ce, err := NewContractExec(cfg, config.LocalParams{})
	require.NoError(t, err)

	err = ce.ContractTxExec(transaction.Transaction{
		Sender:    account.Address(from),
		Recipient: account.EmptyAccountAddr,
		Data:      contractCode,
		Value:     big.NewInt(0),
		GasLimit:  1000000,
	})
	require.NoError(t, err)

	for i := 0; i < 200; i++ {
		err = ce.ContractTxExec(transaction.Transaction{
			Sender:    account.Address(from),
			Recipient: account.Address(contractAddr),
			Data:      setCallData,
			Value:     big.NewInt(0),
			GasLimit:  1000000,
		})
		require.NoError(t, err)

		err = ce.ContractTxExec(transaction.Transaction{
			Sender:    account.Address(from),
			Recipient: account.Address(contractAddr),
			Data:      getCallData,
			Value:     big.NewInt(0),
			GasLimit:  1000000,
		})
		require.NoError(t, err)
	}
}
