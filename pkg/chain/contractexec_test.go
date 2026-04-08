package chain

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

type mockContractExec struct {
	deployFn func(v *vm.Executor, bCtx gethvm.BlockContext, tx transaction.Transaction) (common.Address, uint64, error)
	callFn   func(v *vm.Executor, bCtx gethvm.BlockContext, tx transaction.Transaction) ([]byte, uint64, error)

	deployCalled int
	callCalled   int
}

func (m *mockContractExec) CreateContractTxExecute(
	v *vm.Executor,
	bCtx gethvm.BlockContext,
	tx transaction.Transaction,
) (common.Address, uint64, error) {
	m.deployCalled++
	if m.deployFn == nil {
		return common.Address{}, 0, nil
	}

	return m.deployFn(v, bCtx, tx)
}

func (m *mockContractExec) CallContractTxExecute(
	v *vm.Executor,
	bCtx gethvm.BlockContext,
	tx transaction.Transaction,
) ([]byte, uint64, error) {
	m.callCalled++
	if m.callFn == nil {
		return nil, 0, nil
	}

	return m.callFn(v, bCtx, tx)
}

func TestTxExecute_ContractExec(t *testing.T) {
	var v vm.Executor
	bCtx := gethvm.BlockContext{}

	t.Run("create contract tx dispatched to deploy", func(t *testing.T) {
		mockPort := &mockContractExec{
			deployFn: func(_ *vm.Executor, _ gethvm.BlockContext, tx transaction.Transaction) (common.Address, uint64, error) {
				require.Equal(t, transaction.CreateContractTxType, tx.TxType())
				return common.HexToAddress("0x87e9100fe2b300c290cf0079a058c3450fd86752"), 100, nil
			},
		}
		c := &Chain{contractExec: mockPort}
		tx := transaction.Transaction{
			Sender:    account.Address(common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")),
			Recipient: account.EmptyAccountAddr,
			Data:      common.Hex2Bytes("6000"),
		}

		err := c.txExecute(&v, bCtx, nil, tx)
		require.NoError(t, err)
		require.Equal(t, 1, mockPort.deployCalled)
		require.Equal(t, 0, mockPort.callCalled)
	})

	t.Run("create contract tx wraps deploy error", func(t *testing.T) {
		mockPort := &mockContractExec{
			deployFn: func(_ *vm.Executor, _ gethvm.BlockContext, _ transaction.Transaction) (common.Address, uint64, error) {
				return common.Address{}, 0, errors.New("deploy failed")
			},
		}
		c := &Chain{contractExec: mockPort}
		tx := transaction.Transaction{
			Sender:    account.Address(common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")),
			Recipient: account.EmptyAccountAddr,
			Data:      common.Hex2Bytes("6000"),
		}

		err := c.txExecute(&v, bCtx, nil, tx)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to deploy contract")
	})

	t.Run("call contract tx dispatched to call", func(t *testing.T) {
		mockPort := &mockContractExec{
			callFn: func(_ *vm.Executor, _ gethvm.BlockContext, tx transaction.Transaction) ([]byte, uint64, error) {
				require.Equal(t, transaction.CallContractTxType, tx.TxType())
				return []byte{0x0}, 99, nil
			},
		}
		c := &Chain{contractExec: mockPort}
		tx := transaction.Transaction{
			Sender:    account.Address(common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")),
			Recipient: account.Address(common.HexToAddress("0x87e9100fe2b300c290cf0079a058c3450fd86752")),
			Data:      common.Hex2Bytes("6d4ce63c"),
		}

		err := c.txExecute(&v, bCtx, nil, tx)
		require.NoError(t, err)
		require.Equal(t, 0, mockPort.deployCalled)
		require.Equal(t, 1, mockPort.callCalled)
	})

	t.Run("call contract tx wraps call error", func(t *testing.T) {
		mockPort := &mockContractExec{
			callFn: func(_ *vm.Executor, _ gethvm.BlockContext, _ transaction.Transaction) ([]byte, uint64, error) {
				return nil, 0, errors.New("call failed")
			},
		}
		c := &Chain{contractExec: mockPort}
		tx := transaction.Transaction{
			Sender:    account.Address(common.HexToAddress("0x8bc3d2a374df5e0b9abc0be98210751c0a8df04e")),
			Recipient: account.Address(common.HexToAddress("0x87e9100fe2b300c290cf0079a058c3450fd86752")),
			Data:      common.Hex2Bytes("6d4ce63c"),
		}

		err := c.txExecute(&v, bCtx, nil, tx)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to call contract")
	})
}
