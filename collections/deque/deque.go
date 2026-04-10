// Package deque 提供双端队列实现.
// 注意：本包的数据结构非并发安全，如需在多 goroutine 中使用请自行加锁.
package deque

import "iter"

const minCapacity = 8

// Deque 双端队列.
// 基于环形缓冲区实现，PushFront/PushBack/PopFront/PopBack 操作时间复杂度 O(1).
// 自动扩容和缩容.
// 示例:
//
//	dq := deque.New[int]()
//	dq.PushBack(1)
//	dq.PushBack(2)
//	dq.PushFront(0)
//	// dq: [0, 1, 2]
type Deque[T any] struct {
	buf   []T
	head  int // 第一个元素的索引
	tail  int // 最后一个元素的下一个位置
	count int // 元素数量
}

// New 创建空双端队列.
func New[T any]() *Deque[T] {
	return &Deque[T]{
		buf: make([]T, minCapacity),
	}
}

// NewWithCapacity 创建指定初始容量的双端队列.
func NewWithCapacity[T any](capacity int) *Deque[T] {
	cap := minCapacity
	for cap < capacity {
		cap <<= 1
	}
	return &Deque[T]{
		buf: make([]T, cap),
	}
}

// From 从切片创建双端队列.
func From[T any](items []T) *Deque[T] {
	dq := NewWithCapacity[T](len(items))
	for _, item := range items {
		dq.PushBack(item)
	}
	return dq
}

// PushFront 在头部添加元素.
func (d *Deque[T]) PushFront(item T) {
	d.growIfNeeded()
	d.head = d.prev(d.head)
	d.buf[d.head] = item
	d.count++
}

// PushBack 在尾部添加元素.
func (d *Deque[T]) PushBack(item T) {
	d.growIfNeeded()
	d.buf[d.tail] = item
	d.tail = d.next(d.tail)
	d.count++
}

// PopFront 从头部移除并返回元素.
func (d *Deque[T]) PopFront() (T, bool) {
	if d.count == 0 {
		var zero T
		return zero, false
	}

	item := d.buf[d.head]
	var zero T
	d.buf[d.head] = zero // 帮助 GC
	d.head = d.next(d.head)
	d.count--
	d.shrinkIfNeeded()
	return item, true
}

// PopBack 从尾部移除并返回元素.
func (d *Deque[T]) PopBack() (T, bool) {
	if d.count == 0 {
		var zero T
		return zero, false
	}

	d.tail = d.prev(d.tail)
	item := d.buf[d.tail]
	var zero T
	d.buf[d.tail] = zero // 帮助 GC
	d.count--
	d.shrinkIfNeeded()
	return item, true
}

// PeekFront 查看头部元素（不移除）.
func (d *Deque[T]) PeekFront() (T, bool) {
	if d.count == 0 {
		var zero T
		return zero, false
	}
	return d.buf[d.head], true
}

// PeekBack 查看尾部元素（不移除）.
func (d *Deque[T]) PeekBack() (T, bool) {
	if d.count == 0 {
		var zero T
		return zero, false
	}
	return d.buf[d.prev(d.tail)], true
}

// At 获取指定位置的元素（0 为头部）.
func (d *Deque[T]) At(index int) (T, bool) {
	if index < 0 || index >= d.count {
		var zero T
		return zero, false
	}
	return d.buf[(d.head+index)&(len(d.buf)-1)], true
}

// Set 设置指定位置的元素.
func (d *Deque[T]) Set(index int, item T) bool {
	if index < 0 || index >= d.count {
		return false
	}
	d.buf[(d.head+index)&(len(d.buf)-1)] = item
	return true
}

// Len 返回元素数量.
func (d *Deque[T]) Len() int {
	return d.count
}

// IsEmpty 判断是否为空.
func (d *Deque[T]) IsEmpty() bool {
	return d.count == 0
}

// Clear 清空队列.
func (d *Deque[T]) Clear() {
	var zero T
	for i := range d.buf {
		d.buf[i] = zero
	}
	d.head = 0
	d.tail = 0
	d.count = 0
}

// ToSlice 转换为切片.
func (d *Deque[T]) ToSlice() []T {
	result := make([]T, d.count)
	for i := range d.count {
		result[i] = d.buf[(d.head+i)&(len(d.buf)-1)]
	}
	return result
}

// Clone 复制队列.
func (d *Deque[T]) Clone() *Deque[T] {
	newBuf := make([]T, len(d.buf))
	copy(newBuf, d.buf)
	return &Deque[T]{
		buf:   newBuf,
		head:  d.head,
		tail:  d.tail,
		count: d.count,
	}
}

// ForEach 遍历队列（从头到尾）.
func (d *Deque[T]) ForEach(fn func(T)) {
	for i := range d.count {
		fn(d.buf[(d.head+i)&(len(d.buf)-1)])
	}
}

// ForEachReverse 反向遍历队列（从尾到头）.
func (d *Deque[T]) ForEachReverse(fn func(T)) {
	for i := d.count - 1; i >= 0; i-- {
		fn(d.buf[(d.head+i)&(len(d.buf)-1)])
	}
}

// Rotate 旋转队列.
// n > 0: 向右旋转（头部元素移到尾部）
// n < 0: 向左旋转（尾部元素移到头部）
func (d *Deque[T]) Rotate(n int) {
	if d.count <= 1 {
		return
	}

	n = n % d.count
	if n == 0 {
		return
	}

	if n < 0 {
		n += d.count
	}

	// 向右旋转 n 次
	for range n {
		item, _ := d.PopFront()
		d.PushBack(item)
	}
}

// Reverse 反转队列.
func (d *Deque[T]) Reverse() {
	for i, j := 0, d.count-1; i < j; i, j = i+1, j-1 {
		idxI := (d.head + i) & (len(d.buf) - 1)
		idxJ := (d.head + j) & (len(d.buf) - 1)
		d.buf[idxI], d.buf[idxJ] = d.buf[idxJ], d.buf[idxI]
	}
}

// All 返回从头到尾遍历所有元素的迭代器，产出 (索引, 值).
func (d *Deque[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := range d.count {
			if !yield(i, d.buf[(d.head+i)&(len(d.buf)-1)]) {
				return
			}
		}
	}
}

// Backward 返回从尾到头遍历所有元素的迭代器，产出 (索引, 值).
func (d *Deque[T]) Backward() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := d.count - 1; i >= 0; i-- {
			if !yield(i, d.buf[(d.head+i)&(len(d.buf)-1)]) {
				return
			}
		}
	}
}

// Values 返回从头到尾遍历所有元素值的迭代器.
func (d *Deque[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := range d.count {
			if !yield(d.buf[(d.head+i)&(len(d.buf)-1)]) {
				return
			}
		}
	}
}

// 内部方法

func (d *Deque[T]) next(i int) int {
	return (i + 1) & (len(d.buf) - 1)
}

func (d *Deque[T]) prev(i int) int {
	return (i - 1) & (len(d.buf) - 1)
}

func (d *Deque[T]) growIfNeeded() {
	if d.count == len(d.buf) {
		d.resize(len(d.buf) << 1)
	}
}

func (d *Deque[T]) shrinkIfNeeded() {
	if len(d.buf) > minCapacity && d.count <= len(d.buf)/4 {
		d.resize(len(d.buf) >> 1)
	}
}

func (d *Deque[T]) resize(newSize int) {
	newBuf := make([]T, newSize)

	if d.head < d.tail {
		copy(newBuf, d.buf[d.head:d.tail])
	} else if d.count > 0 {
		n := copy(newBuf, d.buf[d.head:])
		copy(newBuf[n:], d.buf[:d.tail])
	}

	d.buf = newBuf
	d.head = 0
	d.tail = d.count
}
