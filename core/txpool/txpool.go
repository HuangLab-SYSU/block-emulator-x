package txpool

import "github.com/HuangLab-SYSU/block-emulator/core/transaction"

type TxPool interface {
	AddTxs(txs []transaction.Transaction) error
	PackTxsByGivenNum(n int64) ([]transaction.Transaction, error)
	PackTxsByGivenBytes(byteNum int64) ([]transaction.Transaction, error)
}
