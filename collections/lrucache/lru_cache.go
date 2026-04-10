// Package lrucache 提供 LRU (Least Recently Used) 缓存实现.
package lrucache

import (
	"iter"
	"sync"
)

// entry 双向链表节点.
type entry[K comparable, V any] struct {
	key   K
	value V
	prev  *entry[K, V]
	next  *entry[K, V]
}

// LRUCache LRU 缓存.
// 基于哈希表 + 双向链表实现，Get/Put 操作时间复杂度 O(1).
// 当缓存满时，自动淘汰最近最少使用的元素.
// 线程安全.
// 示例:
//
//	cache := lrucache.New[string, int](100)
//	cache.Put("a", 1)
//	cache.Put("b", 2)
//	val, ok := cache.Get("a") // 1, true
type LRUCache[K comparable, V any] struct {
	capacity int
	cache    map[K]*entry[K, V]
	head     *entry[K, V] // 最近使用
	tail     *entry[K, V] // 最久未使用
	mu       sync.RWMutex
}

// New 创建 LRU 缓存.
// capacity 必须大于 0.
func New[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 1
	}

	// 使用哨兵节点简化边界处理
	head := &entry[K, V]{}
	tail := &entry[K, V]{}
	head.next = tail
	tail.prev = head

	return &LRUCache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*entry[K, V], capacity),
		head:     head,
		tail:     tail,
	}
}

// Get 获取缓存值.
// 如果键存在，会将其移动到最近使用位置.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.cache[key]; ok {
		c.moveToFront(e)
		return e.value, true
	}

	var zero V
	return zero, false
}

// Put 设置缓存值.
// 如果键已存在，更新值并移动到最近使用位置.
// 如果缓存满，淘汰最久未使用的元素.
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.cache[key]; ok {
		e.value = value
		c.moveToFront(e)
		return
	}

	// 缓存满，淘汰最久未使用
	if len(c.cache) >= c.capacity {
		c.removeLast()
	}

	// 添加新节点
	e := &entry[K, V]{key: key, value: value}
	c.cache[key] = e
	c.addToFront(e)
}

// GetOrPut 获取缓存值，不存在则调用 loader 加载并缓存.
func (c *LRUCache[K, V]) GetOrPut(key K, loader func() V) V {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.cache[key]; ok {
		c.moveToFront(e)
		return e.value
	}

	value := loader()

	if len(c.cache) >= c.capacity {
		c.removeLast()
	}

	e := &entry[K, V]{key: key, value: value}
	c.cache[key] = e
	c.addToFront(e)

	return value
}

// Peek 查看缓存值（不影响 LRU 顺序）.
func (c *LRUCache[K, V]) Peek(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if e, ok := c.cache[key]; ok {
		return e.value, true
	}

	var zero V
	return zero, false
}

// Contains 判断键是否存在（不影响 LRU 顺序）.
func (c *LRUCache[K, V]) Contains(key K) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.cache[key]
	return ok
}

// Remove 删除缓存项.
func (c *LRUCache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.cache[key]; ok {
		c.removeEntry(e)
		delete(c.cache, key)
		return true
	}
	return false
}

// Len 返回当前缓存数量.
func (c *LRUCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Capacity 返回缓存容量.
func (c *LRUCache[K, V]) Capacity() int {
	return c.capacity
}

// Clear 清空缓存.
func (c *LRUCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[K]*entry[K, V], c.capacity)
	c.head.next = c.tail
	c.tail.prev = c.head
}

// Keys 返回所有键（按最近使用顺序）.
func (c *LRUCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]K, 0, len(c.cache))
	for e := c.head.next; e != c.tail; e = e.next {
		keys = append(keys, e.key)
	}
	return keys
}

// Resize 调整缓存容量.
// 如果新容量小于当前元素数量，会淘汰多余元素.
func (c *LRUCache[K, V]) Resize(capacity int) {
	if capacity <= 0 {
		capacity = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.capacity = capacity

	// 淘汰多余元素
	for len(c.cache) > capacity {
		c.removeLast()
	}
}

// All 返回按最近使用顺序遍历所有键值对的迭代器（最近使用在前）.
// 遍历期间持有读锁，不会影响 LRU 顺序.
func (c *LRUCache[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		c.mu.RLock()
		defer c.mu.RUnlock()
		for e := c.head.next; e != c.tail; e = e.next {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

// 内部方法（调用前需持有锁）

// addToFront 添加节点到头部.
func (c *LRUCache[K, V]) addToFront(e *entry[K, V]) {
	e.prev = c.head
	e.next = c.head.next
	c.head.next.prev = e
	c.head.next = e
}

// removeEntry 从链表中移除节点.
func (c *LRUCache[K, V]) removeEntry(e *entry[K, V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
}

// moveToFront 移动节点到头部.
func (c *LRUCache[K, V]) moveToFront(e *entry[K, V]) {
	c.removeEntry(e)
	c.addToFront(e)
}

// removeLast 移除最后一个节点（最久未使用）.
func (c *LRUCache[K, V]) removeLast() {
	last := c.tail.prev
	if last != c.head {
		c.removeEntry(last)
		delete(c.cache, last.key)
	}
}
