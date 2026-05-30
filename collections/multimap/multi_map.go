// Package multimap 提供一对多映射MultiMap实现.
// 注意：本包的数据结构非并发安全，如需在多 goroutine 中使用请自行加锁.
package multimap

import "iter"

// MultiMap 一个键对应多个值的 Map.
// 零值不可用，需通过 New 创建.
type MultiMap[K comparable, V any] struct {
	m    map[K][]V
	size int // 总键值对数
}

// New 创建空 MultiMap.
func New[K comparable, V any]() *MultiMap[K, V] {
	return &MultiMap[K, V]{m: make(map[K][]V)}
}

// Put 向 key 追加一个值.
func (m *MultiMap[K, V]) Put(key K, value V) {
	m.m[key] = append(m.m[key], value)
	m.size++
}

// PutAll 向 key 追加多个值.
func (m *MultiMap[K, V]) PutAll(key K, values ...V) {
	m.m[key] = append(m.m[key], values...)
	m.size += len(values)
}

// Get 返回 key 对应的所有值的副本，不存在时返回 nil.
// 返回的切片是内部数据的拷贝，修改不会影响 MultiMap.
func (m *MultiMap[K, V]) Get(key K) []V {
	vals := m.m[key]
	if vals == nil {
		return nil
	}
	cp := make([]V, len(vals))
	copy(cp, vals)
	return cp
}

// Remove 移除整个 key 及其所有值，返回是否成功.
func (m *MultiMap[K, V]) Remove(key K) bool {
	vals, ok := m.m[key]
	if !ok {
		return false
	}
	m.size -= len(vals)
	delete(m.m, key)
	return true
}

// RemoveValue 移除 key 下的特定值仅移除第一次出现，返回是否成功.
// V 需满足 comparable 约束，通过独立的泛型函数实现以绕过 Go 泛型限制.
func RemoveValue[K comparable, V comparable](m *MultiMap[K, V], key K, value V) bool {
	vals, ok := m.m[key]
	if !ok {
		return false
	}
	for i, v := range vals {
		if v == value {
			// 创建新切片，避免修改外部持有者可能保留的旧切片
			newVals := make([]V, 0, len(vals)-1)
			newVals = append(newVals, vals[:i]...)
			newVals = append(newVals, vals[i+1:]...)
			m.m[key] = newVals
			m.size--
			if len(newVals) == 0 {
				delete(m.m, key)
			}
			return true
		}
	}
	return false
}

// ContainsKey 判断是否包含指定键.
func (m *MultiMap[K, V]) ContainsKey(key K) bool {
	_, ok := m.m[key]
	return ok
}

// Keys 返回所有键顺序不确定.
func (m *MultiMap[K, V]) Keys() []K {
	result := make([]K, 0, len(m.m))
	for k := range m.m {
		result = append(result, k)
	}
	return result
}

// Values 返回所有值展开为一维切片顺序不确定.
func (m *MultiMap[K, V]) Values() []V {
	result := make([]V, 0, m.size)
	for _, vals := range m.m {
		result = append(result, vals...)
	}
	return result
}

// Len 返回总键值对数所有键的值数量之和.
func (m *MultiMap[K, V]) Len() int {
	return m.size
}

// KeyLen 返回键的数量.
func (m *MultiMap[K, V]) KeyLen() int {
	return len(m.m)
}

// Range 遍历所有键值对key -> 该键所有值切片；fn 返回 false 时停止.
func (m *MultiMap[K, V]) Range(fn func(K, []V) bool) {
	for k, vals := range m.m {
		if !fn(k, vals) {
			return
		}
	}
}

// Clear 清空所有元素.
func (m *MultiMap[K, V]) Clear() {
	m.m = make(map[K][]V)
	m.size = 0
}

// All 返回遍历所有键及其值切片的迭代器顺序不确定.
func (m *MultiMap[K, V]) All() iter.Seq2[K, []V] {
	return func(yield func(K, []V) bool) {
		for k, vals := range m.m {
			if !yield(k, vals) {
				return
			}
		}
	}
}
