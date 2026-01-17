package chain

import (
	"fmt"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/vm"
)

const (
	// blockNumBias increase the number of block, so that the contract can be called correctly.
	blockNumBias  = 20_000_000
	blockGasLimit = 1000000000000
)

var initBalance, _ = uint256.FromDecimal(account.InitBalanceStr)

func getBlockCtxByBlock(b *block.Block) gethvm.BlockContext {
	return gethvm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(n uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address(b.Miner),
		BlockNumber: new(big.Int).SetUint64(b.Number + blockNumBias),
		Time:        uint64(b.CreateTime.Unix()),
		Difficulty:  common.Big0,
		GasLimit:    blockGasLimit,
		BaseFee:     big.NewInt(0),
	}
}

func normalTxExecute(v *vm.Executor, tx transaction.Transaction) error {
	uVal, err := utils.BigToUInt256(tx.Value)
	if err != nil {
		return fmt.Errorf("convert value to uint256 failed: %w", err)
	}

	s := v.StateDB()
	sAddr, rAddr := common.Address(tx.Sender), common.Address(tx.Recipient)

	setInitBalanceIfNotExist(s, sAddr, rAddr)

	if !core.CanTransfer(s, sAddr, uVal) {
		return fmt.Errorf("transfer failed: the balance of %x is not enough", tx.Sender)
	}

	s.SubBalance(sAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))
	s.AddBalance(rAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))

	return nil
}

func relayTxExecute(v *vm.Executor, addrLoc map[account.Address]int64, shard int64, tx transaction.Transaction) error {
	uVal, err := utils.BigToUInt256(tx.Value)
	if err != nil {
		return fmt.Errorf("convert value to uint256 failed: %w", err)
	}

	s := v.StateDB()

	switch tx.RelayStage {
	case transaction.Relay1Tx:
		// For a relay1 transaction, debit the sender's balance.
		if addrLoc[tx.Sender] != shard {
			// Sender is not in this shard, skip.
			return nil
		}

		sAddr := common.Address(tx.Sender)
		setInitBalanceIfNotExist(s, sAddr)

		if !core.CanTransfer(s, sAddr, uVal) {
			return fmt.Errorf("transfer failed: the balance of %x is not enough", tx.Sender)
		}

		s.SubBalance(sAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))
	case transaction.Relay2Tx:
		// For a relay2 transaction credit the recipient's balance.
		if addrLoc[tx.Recipient] != shard {
			return nil
		}

		rAddr := common.Address(tx.Recipient)
		setInitBalanceIfNotExist(s, rAddr)

		s.AddBalance(rAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))
	default:
		return fmt.Errorf("unknown relay transaction type: %b", tx.RelayStage)
	}

	return nil
}

func brokerTxExecute(v *vm.Executor, addrLoc map[account.Address]int64, shard int64, tx transaction.Transaction) error {
	uVal, err := utils.BigToUInt256(tx.Value)
	if err != nil {
		return fmt.Errorf("convert value to uint256 failed: %w", err)
	}

	s := v.StateDB()

	switch tx.BrokerStage {
	case transaction.Sigma1BrokerStage:
		// For a broker1 transaction, debit the sender's balance and credit the broker's balance.
		if addrLoc[tx.Sender] != shard {
			slog.Error("handle broker1 tx error", "err", "the sender should be in this shard but not")
			return nil
		}

		sAddr, bAddr := common.Address(tx.Sender), common.Address(tx.Broker)
		setInitBalanceIfNotExist(s, sAddr, bAddr)

		if !core.CanTransfer(s, sAddr, uVal) {
			return fmt.Errorf("transfer failed: the balance of %x is not enough", tx.Sender)
		}

		s.SubBalance(sAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))
		s.AddBalance(bAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))
	case transaction.Sigma2BrokerStage:
		// For a broker2 transaction, debit the broker's balance and credit the recipient's balance.
		if addrLoc[tx.Recipient] != shard {
			slog.Error("handle broker2 tx error", "err", "the recipient should be in this shard but not")
			return nil
		}

		rAddr := common.Address(tx.Recipient)
		setInitBalanceIfNotExist(s, rAddr)

		s.AddBalance(rAddr, uVal, tracing.BalanceChangeReason(stateReasonTransaction))

	default:
		return fmt.Errorf("unknown broker transaction type: %b", tx.BrokerStage)
	}

	return nil
}

func setInitBalanceIfNotExist(s *state.StateDB, addresses ...common.Address) {
	for _, address := range addresses {
		if !s.Exist(address) {
			s.SetBalance(address, initBalance, tracing.BalanceChangeReason(stateReasonBalanceInit))
		}
	}
}
