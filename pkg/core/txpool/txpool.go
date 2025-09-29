package txpool

import (
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

type TxPool interface {
	AddTxs(txs []transaction.Transaction) error
	PackTxs() ([]transaction.Transaction, error)
}
