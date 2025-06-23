package queue

import (
	"github.com/HuangLab-SYSU/block-emulator/core/transaction"
	"sync"
)

type TxPool struct {
	Queue []transaction.Transaction
	lock  sync.Mutex
}

func NewTxPool() *TxPool {
	return &TxPool{
		Queue: make([]transaction.Transaction, 0),
	}
}

func (t *TxPool) AddTxs(txs []transaction.Transaction) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	for _, tx := range txs {
		t.Queue = append(t.Queue, tx)
	}
	return nil
}

func (t *TxPool) PackTxsByGivenNum(n int) ([]transaction.Transaction, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	length := len(t.Queue)
	if length > n {
		length = n
	}
	ret := make([]transaction.Transaction, length)
	copy(ret, t.Queue[:length]) // deep copy
	t.Queue = t.Queue[length:]
	return ret, nil
}

func (t *TxPool) PackTxsByGivenBytes(byteNum int) ([]transaction.Transaction, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	endIdx := 0
	for i, tx := range t.Queue {
		b, err := tx.Encode()
		if err != nil {
			return nil, err
		}
		size := len(b)
		if size < byteNum {
			break
		}
		byteNum -= size
		endIdx = i + 1
	}

	ret := make([]transaction.Transaction, endIdx)
	copy(ret, t.Queue[:endIdx]) // deep copy
	t.Queue = t.Queue[endIdx:]
	return ret, nil
}
