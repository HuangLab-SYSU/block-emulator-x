package txpool

import (
	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/txpool/queue"
)

// TxPool is a pool that buffers transaction.
type TxPool interface {
	// AddTxs adds the given transactions into the pool.
	AddTxs(txs []transaction.Transaction) error
	// PackTxs pops transactions from the pool.
	// The size of transactions will be limited by the given parameter 'limit'.
	PackTxs(limit int) ([]transaction.Transaction, error)
	// GetTxListSize returns the size of the given tx list.
	GetTxListSize(txs []transaction.Transaction) (int, error)
}

func NewTxPool(cfg config.TxPoolCfg) (TxPool, error) {
	return queue.NewTxPool(cfg)
}
