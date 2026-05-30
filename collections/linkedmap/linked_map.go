// Package linkedmap 提供维护插入顺序的 Map 实现.
// 注意：本包的数据结构非并发安全，如需在多 goroutine 中使用请自行加锁.
package linkedmap

import "iter"

// entry 双向链表节点.
type entry[K comparable, V any] struct {
	key        K
	value      V
	prev, next *entry[K, V]
}

// LinkedMap 维护插入顺序的 Map哈希表 + 双向链表.
// 零值不可用，需通过 New 创建.
type LinkedMap[K comparable, V any] struct {
	table map[K]*entry[K, V]
	head  *entry[K, V] // 哨兵头节点
	tail  *entry[K, V] // 哨兵尾节点
}

// New 创建空 LinkedMap.
func New[K comparable, V any]() *LinkedMap[K, V] {
	head := &entry[K, V]{}
	tail := &entry[K, V]{}
	head.next = tail
	tail.prev = head
	return &LinkedMap[K, V]{
		table: make(map[K]*entry[K, V]),
		head:  head,
		tail:  tail,
	}
}

// Put 插入或更新键值对.
// 若键已存在则更新值不改变顺序；否则追加到末尾.
func (m *LinkedMap[K, V]) Put(key K, value V) {
	if e, ok := m.table[key]; ok {
		e.value = value
		return
	}
	e := &entry[K, V]{key: key, value: value}
	m.table[key] = e
	// 追加到链表尾部tail 哨兵之前
	e.prev = m.tail.prev
	e.next = m.tail
	m.tail.prev.next = e
	m.tail.prev = e
}

// Get 根据键获取值.
func (m *LinkedMap[K, V]) Get(key K) (V, bool) {
	if e, ok := m.table[key]; ok {
		return e.value, true
	}
	var zero V
	return zero, false
}

// Remove 删除指定键，返回是否成功.
func (m *LinkedMap[K, V]) Remove(key K) bool {
	e, ok := m.table[key]
	if !ok {
		return false
	}
	delete(m.table, key)
	e.prev.next = e.next
	e.next.prev = e.prev
	return true
}

// ContainsKey 判断是否包含指定键.
func (m *LinkedMap[K, V]) ContainsKey(key K) bool {
	_, ok := m.table[key]
	return ok
}

// Len 返回键值对数量.
func (m *LinkedMap[K, V]) Len() int {
	return len(m.table)
}

// Keys 按插入顺序返回所有键.
func (m *LinkedMap[K, V]) Keys() []K {
	result := make([]K, 0, len(m.table))
	for e := m.head.next; e != m.tail; e = e.next {
		result = append(result, e.key)
	}
	return result
}

// Values 按插入顺序返回所有值.
func (m *LinkedMap[K, V]) Values() []V {
	result := make([]V, 0, len(m.table))
	for e := m.head.next; e != m.tail; e = e.next {
		result = append(result, e.value)
	}
	return result
}

// Range 按插入顺序遍历所有键值对，fn 返回 false 时停止遍历.
func (m *LinkedMap[K, V]) Range(fn func(K, V) bool) {
	for e := m.head.next; e != m.tail; e = e.next {
		if !fn(e.key, e.value) {
			return
		}
	}
}

// Clear 清空所有元素.
func (m *LinkedMap[K, V]) Clear() {
	m.table = make(map[K]*entry[K, V])
	m.head.next = m.tail
	m.tail.prev = m.head
}

// All 返回按插入顺序遍历所有键值对的迭代器.
func (m *LinkedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for e := m.head.next; e != m.tail; e = e.next {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

// Backward 返回按插入逆序遍历所有键值对的迭代器.
func (m *LinkedMap[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for e := m.tail.prev; e != m.head; e = e.prev {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}
