package queue

import (
	"fmt"
	"sync"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

type packTxFunc func(q []transaction.Transaction, n int64) ([]transaction.Transaction, []transaction.Transaction, error)

type TxPool struct {
	queue []transaction.Transaction
	limit int64
	pf    packTxFunc
	lock  sync.Mutex
}

func NewTxPool(cfg config.TxPoolCfg) (*TxPool, error) {
	var pf packTxFunc

	switch cfg.Type {
	case "byte":
		pf = packTxsByGivenBytes
	case "number":
		pf = packTxsByGivenNum
	default:
		return nil, fmt.Errorf("unknown tx pool type: %s", cfg.Type)
	}

	return &TxPool{
		queue: make([]transaction.Transaction, 0),
		pf:    pf,
		limit: cfg.Limit,
	}, nil
}

func (t *TxPool) AddTxs(txs []transaction.Transaction) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.queue = append(t.queue, txs...)

	return nil
}

func (t *TxPool) PackTxs() ([]transaction.Transaction, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var (
		packed, q []transaction.Transaction
		err       error
	)

	packed, q, err = t.pf(t.queue, t.limit)
	if err != nil {
		return nil, fmt.Errorf("call pack tx func err: %w", err)
	}

	t.queue = q

	return packed, nil
}

func packTxsByGivenNum(q []transaction.Transaction, n int64) ([]transaction.Transaction, []transaction.Transaction, error) {
	length := int64(len(q))
	if length > n {
		length = n
	}

	ret := make([]transaction.Transaction, length)
	copy(ret, q[:length]) // deep copy
	q = q[length:]

	return ret, q, nil
}

func packTxsByGivenBytes(q []transaction.Transaction, n int64) ([]transaction.Transaction, []transaction.Transaction, error) {
	endIdx := 0

	for i, tx := range q {
		b, err := tx.Encode()
		if err != nil {
			return nil, nil, err
		}

		size := int64(len(b))
		if size < n {
			break
		}

		n -= size
		endIdx = i + 1
	}

	ret := make([]transaction.Transaction, endIdx)
	copy(ret, q[:endIdx]) // deep copy
	q = q[endIdx:]

	return ret, q, nil
}
