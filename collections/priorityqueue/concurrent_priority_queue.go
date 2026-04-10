package priorityqueue

import (
	"cmp"
	"iter"
	"sync"
)

// ConcurrentPriorityQueue 线程安全的优先队列.
type ConcurrentPriorityQueue[T any] struct {
	pq *PriorityQueue[T]
	mu sync.RWMutex
}

// NewConcurrent 创建线程安全的优先队列.
func NewConcurrent[T any](less LessFunc[T]) *ConcurrentPriorityQueue[T] {
	return &ConcurrentPriorityQueue[T]{pq: New(less)}
}

// NewConcurrentMin 创建线程安全的最小优先队列.
func NewConcurrentMin[T cmp.Ordered]() *ConcurrentPriorityQueue[T] {
	return &ConcurrentPriorityQueue[T]{pq: NewMin[T]()}
}

// NewConcurrentMax 创建线程安全的最大优先队列.
func NewConcurrentMax[T cmp.Ordered]() *ConcurrentPriorityQueue[T] {
	return &ConcurrentPriorityQueue[T]{pq: NewMax[T]()}
}

// Push 向队列中添加元素.
func (cpq *ConcurrentPriorityQueue[T]) Push(items ...T) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	cpq.pq.Push(items...)
}

// Pop 弹出并返回优先级最高的元素.
func (cpq *ConcurrentPriorityQueue[T]) Pop() (T, bool) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.Pop()
}

// Peek 查看优先级最高的元素但不弹出.
func (cpq *ConcurrentPriorityQueue[T]) Peek() (T, bool) {
	cpq.mu.RLock()
	defer cpq.mu.RUnlock()
	return cpq.pq.Peek()
}

// Len 返回队列中的元素数量.
func (cpq *ConcurrentPriorityQueue[T]) Len() int {
	cpq.mu.RLock()
	defer cpq.mu.RUnlock()
	return cpq.pq.Len()
}

// IsEmpty 判断队列是否为空.
func (cpq *ConcurrentPriorityQueue[T]) IsEmpty() bool {
	cpq.mu.RLock()
	defer cpq.mu.RUnlock()
	return cpq.pq.IsEmpty()
}

// Clear 清空队列.
func (cpq *ConcurrentPriorityQueue[T]) Clear() {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	cpq.pq.Clear()
}

// ToSlice 按优先级顺序弹出所有元素，会清空队列.
func (cpq *ConcurrentPriorityQueue[T]) ToSlice() []T {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.ToSlice()
}

// Clone 返回队列的深拷贝.
func (cpq *ConcurrentPriorityQueue[T]) Clone() *ConcurrentPriorityQueue[T] {
	cpq.mu.RLock()
	defer cpq.mu.RUnlock()
	return &ConcurrentPriorityQueue[T]{pq: cpq.pq.Clone()}
}

// All 返回按优先级顺序遍历所有元素的迭代器.
// 内部持有读锁并使用克隆队列，不会修改原队列.
func (cpq *ConcurrentPriorityQueue[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		cpq.mu.RLock()
		clone := cpq.pq.Clone()
		cpq.mu.RUnlock()
		for clone.Len() > 0 {
			item, _ := clone.Pop()
			if !yield(item) {
				return
			}
		}
	}
}
