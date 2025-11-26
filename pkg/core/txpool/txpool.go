package txpool

import (
	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool/queue"
)

type TxPool interface {
	AddTxs(txs []transaction.Transaction) error
	PackTxs(limit int) ([]transaction.Transaction, error)
}

func NewTxPool(cfg config.TxPoolCfg) (TxPool, error) {
	return queue.NewTxPool(cfg)
}
