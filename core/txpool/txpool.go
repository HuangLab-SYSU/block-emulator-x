package txpool

import "github.com/HuangLab-SYSU/block-emulator/core/transaction"

type TxPool interface {
	AddTxs(txs []transaction.Transaction) error
	PackTxsByGivenNum(n int) ([]transaction.Transaction, error)
	PackTxsByGivenBytes(byteNum int) ([]transaction.Transaction, error)
}
