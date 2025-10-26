package txsource

import "github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"

type TxSource interface {
	ReadTxs(size int64) ([]transaction.Transaction, error)
}

type NoOperationTxSource struct{}

func (NoOperationTxSource) ReadTxs(size int64) ([]transaction.Transaction, error) {
	return nil, nil
}
