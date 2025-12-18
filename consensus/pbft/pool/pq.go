package pool

import (
	"container/heap"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
)

// LessFunc 定义比较函数类型
type LessFunc[T any] func(a, b T) bool

// PriorityQueue is the priority queue for PBFT messages.
type PriorityQueue[T any] struct {
	items []T
	less  LessFunc[T]
}

func NewPQ[T any](less LessFunc[T]) *PriorityQueue[T] {
	pq := &PriorityQueue[T]{less: less}
	heap.Init(pq)

	return pq
}

func (pq *PriorityQueue[T]) Len() int { return len(pq.items) }

func (pq *PriorityQueue[T]) Less(i, j int) bool {
	return pq.less(pq.items[i], pq.items[j])
}

func (pq *PriorityQueue[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

func (pq *PriorityQueue[T]) Push(x any) {
	pq.items = append(pq.items, x.(T))
}

func (pq *PriorityQueue[T]) Pop() any {
	old := pq.items
	n := len(old)
	x := old[n-1]
	pq.items = old[:n-1]

	return x
}

func (pq *PriorityQueue[T]) PushItem(x T) {
	heap.Push(pq, x)
}

func (pq *PriorityQueue[T]) PopItem() T {
	return heap.Pop(pq).(T)
}

func PreprepareMsgLess(a, b *message.PreprepareMsg) bool {
	if a.View == b.View {
		return a.Seq < b.Seq
	}

	return a.View < b.View
}

func PrepareMsgLess(a, b *message.PrepareMsg) bool {
	if a.View == b.View {
		return a.Seq < b.Seq
	}

	return a.View < b.View
}

func CommitMsgLess(a, b *message.CommitMsg) bool {
	if a.View == b.View {
		return a.Seq < b.Seq
	}

	return a.View < b.View
}
