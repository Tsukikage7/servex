// Package priorityqueue 提供基于堆实现的优先队列.
// 注意：PriorityQueue 非并发安全，如需在多 goroutine 中使用请使用 ConcurrentPriorityQueue 或自行加锁.
package priorityqueue

import (
	"cmp"
	"iter"
)

// LessFunc 比较函数，返回 true 表示 a 优先级高于 b.
type LessFunc[T any] func(a, b T) bool

// PriorityQueue 优先队列.
// 基于二叉堆实现，支持 O(log n) 的插入和弹出操作.
// 示例:
//
//	// 最小堆
//	pq := priorityqueue.NewMin[int]()
//	pq.Push(3, 1, 2)
//	pq.Pop() // 1
//	// 最大堆
//	pq := priorityqueue.NewMax[int]()
//	pq.Push(3, 1, 2)
//	pq.Pop() // 3
//	// 自定义优先级
//	pq := priorityqueue.New(func(a, b Task) bool {
//	    return a.Priority > b.Priority
//	})
type PriorityQueue[T any] struct {
	data []T
	less LessFunc[T]
}

// New 创建优先队列，需要提供比较函数.
// less(a, b) 返回 true 表示 a 应该排在 b 前面.
func New[T any](less LessFunc[T]) *PriorityQueue[T] {
	return &PriorityQueue[T]{
		data: make([]T, 0),
		less: less,
	}
}

// NewMin 创建最小堆（小的优先）.
func NewMin[T cmp.Ordered]() *PriorityQueue[T] {
	return New(func(a, b T) bool { return a < b })
}

// NewMax 创建最大堆（大的优先）.
func NewMax[T cmp.Ordered]() *PriorityQueue[T] {
	return New(func(a, b T) bool { return a > b })
}

// Push 添加元素.
func (pq *PriorityQueue[T]) Push(items ...T) {
	for _, item := range items {
		pq.data = append(pq.data, item)
		pq.up(len(pq.data) - 1)
	}
}

// Pop 弹出优先级最高的元素.
func (pq *PriorityQueue[T]) Pop() (T, bool) {
	if len(pq.data) == 0 {
		var zero T
		return zero, false
	}

	top := pq.data[0]
	last := len(pq.data) - 1
	pq.data[0] = pq.data[last]
	var zero T
	pq.data[last] = zero // 清零移除位置，帮助 GC 回收
	pq.data = pq.data[:last]

	if len(pq.data) > 0 {
		pq.down(0)
	}

	return top, true
}

// Peek 查看优先级最高的元素（不弹出）.
func (pq *PriorityQueue[T]) Peek() (T, bool) {
	if len(pq.data) == 0 {
		var zero T
		return zero, false
	}
	return pq.data[0], true
}

// Len 返回元素数量.
func (pq *PriorityQueue[T]) Len() int {
	return len(pq.data)
}

// IsEmpty 判断是否为空.
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return len(pq.data) == 0
}

// Clear 清空所有元素.
func (pq *PriorityQueue[T]) Clear() {
	pq.data = pq.data[:0]
}

// DrainToSlice 按优先级顺序弹出所有元素并返回切片.
// 注意：此操作是破坏性的，会清空队列.
func (pq *PriorityQueue[T]) DrainToSlice() []T {
	result := make([]T, 0, len(pq.data))
	for pq.Len() > 0 {
		item, _ := pq.Pop()
		result = append(result, item)
	}
	return result
}

// ToSlice 返回所有元素（按优先级顺序弹出）.
// Deprecated: 此方法会清空队列，请使用 DrainToSlice 替代.
func (pq *PriorityQueue[T]) ToSlice() []T {
	return pq.DrainToSlice()
}

// Clone 克隆优先队列.
func (pq *PriorityQueue[T]) Clone() *PriorityQueue[T] {
	clone := &PriorityQueue[T]{
		data: make([]T, len(pq.data)),
		less: pq.less,
	}
	copy(clone.data, pq.data)
	return clone
}

// All 返回按优先级顺序遍历所有元素的迭代器.
// 内部使用克隆队列逐个弹出，不会修改原队列.
func (pq *PriorityQueue[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		clone := pq.Clone()
		for clone.Len() > 0 {
			item, _ := clone.Pop()
			if !yield(item) {
				return
			}
		}
	}
}

// up 向上调整堆.
func (pq *PriorityQueue[T]) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !pq.less(pq.data[i], pq.data[parent]) {
			break
		}
		pq.data[i], pq.data[parent] = pq.data[parent], pq.data[i]
		i = parent
	}
}

// down 向下调整堆.
func (pq *PriorityQueue[T]) down(i int) {
	n := len(pq.data)
	for {
		left := 2*i + 1
		if left >= n {
			break
		}

		// 找到优先级最高的子节点
		smallest := left
		right := left + 1
		if right < n && pq.less(pq.data[right], pq.data[left]) {
			smallest = right
		}

		// 如果当前节点优先级更高，停止
		if pq.less(pq.data[i], pq.data[smallest]) {
			break
		}

		pq.data[i], pq.data[smallest] = pq.data[smallest], pq.data[i]
		i = smallest
	}
}
