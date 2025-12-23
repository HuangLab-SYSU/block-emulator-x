package queue

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
)

type packTxFunc func(q []transaction.Transaction, n int) ([]transaction.Transaction, []transaction.Transaction, error)

type TxPool struct {
	queue []transaction.Transaction
	pf    packTxFunc
	lock  sync.Mutex
	cfg   config.TxPoolCfg
}

func NewTxPool(cfg config.TxPoolCfg) (*TxPool, error) {
	var pf packTxFunc

	switch cfg.Type {
	case config.TxPoolByteType:
		pf = packTxsByGivenBytes
	case config.TxPoolNumType:
		pf = packTxsByGivenNum
	default:
		return nil, fmt.Errorf("unknown tx pool type: %s", cfg.Type)
	}

	return &TxPool{
		queue: make([]transaction.Transaction, 0),
		pf:    pf,
		cfg:   cfg,
	}, nil
}

func (t *TxPool) AddTxs(txs []transaction.Transaction) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.queue = append(t.queue, txs...)

	return nil
}

func (t *TxPool) PackTxs(limit int) ([]transaction.Transaction, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var (
		packed, q []transaction.Transaction
		err       error
	)

	packed, q, err = t.pf(t.queue, limit)
	if err != nil {
		return nil, fmt.Errorf("call pack tx func err: %w", err)
	}

	t.queue = q

	slog.Debug("txs are packed from the tx pool", "packed tx size", len(packed), "tx pool size", len(q))

	return packed, nil
}

func (t *TxPool) GetTxListSize(txs []transaction.Transaction) (int, error) {
	switch t.cfg.Type {
	case config.TxPoolByteType:
		return getCurSizeOfByte(txs)
	case config.TxPoolNumType:
		return getCurSizeOfNum(txs)
	}

	return 0, fmt.Errorf("unknown tx pool type: %s", t.cfg.Type)
}

func packTxsByGivenNum(
	q []transaction.Transaction,
	n int,
) ([]transaction.Transaction, []transaction.Transaction, error) {
	length := len(q)
	if length > n {
		length = n
	}

	ret := make([]transaction.Transaction, length)
	copy(ret, q[:length]) // deep copy
	q = q[length:]

	return ret, q, nil
}

func packTxsByGivenBytes(
	q []transaction.Transaction,
	n int,
) ([]transaction.Transaction, []transaction.Transaction, error) {
	endIdx := 0

	for i, tx := range q {
		b, err := tx.Encode()
		if err != nil {
			return nil, nil, err
		}

		size := len(b)
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

func getCurSizeOfNum(txs []transaction.Transaction) (int, error) {
	return len(txs), nil
}

func getCurSizeOfByte(txs []transaction.Transaction) (int, error) {
	cnt := 0

	for _, tx := range txs {
		b, err := tx.Encode()
		if err != nil {
			return 0, err
		}

		cnt += len(b)
	}

	return cnt, nil
}
