// Package hashset 提供基于 map 实现的无序集合.
// 注意：本包的数据结构非并发安全，如需在多 goroutine 中使用请自行加锁.
package hashset

import "iter"

// HashSet 基于 map 的无序集合.
// 特性:
//   - Add/Remove/Contains 操作时间复杂度 O(1)
//   - 不保证元素顺序
//   - 不允许重复元素
// 示例:
//	hs := hashset.New(1, 2, 3)
//	hs.Add(4)
//	hs.Contains(1) // true
type HashSet[T comparable] struct {
	m map[T]struct{}
}

// New 创建 HashSet.
func New[T comparable](items ...T) *HashSet[T] {
	s := &HashSet[T]{m: make(map[T]struct{}, len(items))}
	s.Add(items...)
	return s
}

// FromSlice 从切片创建 HashSet.
func FromSlice[T comparable](items []T) *HashSet[T] {
	return New(items...)
}

// Add 添加元素.
func (s *HashSet[T]) Add(items ...T) {
	for _, item := range items {
		s.m[item] = struct{}{}
	}
}

// Remove 移除元素.
func (s *HashSet[T]) Remove(items ...T) {
	for _, item := range items {
		delete(s.m, item)
	}
}

// Contains 判断元素是否存在.
func (s *HashSet[T]) Contains(item T) bool {
	_, ok := s.m[item]
	return ok
}

// Len 返回元素数量.
func (s *HashSet[T]) Len() int {
	return len(s.m)
}

// IsEmpty 判断是否为空.
func (s *HashSet[T]) IsEmpty() bool {
	return len(s.m) == 0
}

// Clear 清空所有元素.
func (s *HashSet[T]) Clear() {
	s.m = make(map[T]struct{})
}

// ToSlice 返回所有元素（顺序不确定）.
func (s *HashSet[T]) ToSlice() []T {
	result := make([]T, 0, len(s.m))
	for item := range s.m {
		result = append(result, item)
	}
	return result
}

// Range 遍历所有元素（顺序不确定）.
// fn 返回 false 时停止遍历.
func (s *HashSet[T]) Range(fn func(item T) bool) {
	for item := range s.m {
		if !fn(item) {
			return
		}
	}
}

// Clone 克隆 HashSet.
func (s *HashSet[T]) Clone() *HashSet[T] {
	clone := &HashSet[T]{m: make(map[T]struct{}, len(s.m))}
	for item := range s.m {
		clone.m[item] = struct{}{}
	}
	return clone
}

// Union 返回与另一个集合的并集.
func (s *HashSet[T]) Union(other *HashSet[T]) *HashSet[T] {
	result := s.Clone()
	for item := range other.m {
		result.m[item] = struct{}{}
	}
	return result
}

// Intersection 返回与另一个集合的交集.
func (s *HashSet[T]) Intersection(other *HashSet[T]) *HashSet[T] {
	result := New[T]()

	// 遍历较小的集合
	smaller, larger := s, other
	if s.Len() > other.Len() {
		smaller, larger = other, s
	}

	for item := range smaller.m {
		if larger.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

// Difference 返回差集（s - other）.
func (s *HashSet[T]) Difference(other *HashSet[T]) *HashSet[T] {
	result := New[T]()
	for item := range s.m {
		if !other.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

// SymmetricDifference 返回对称差集.
// 包含只在其中一个集合中出现的元素.
func (s *HashSet[T]) SymmetricDifference(other *HashSet[T]) *HashSet[T] {
	result := New[T]()
	for item := range s.m {
		if !other.Contains(item) {
			result.Add(item)
		}
	}
	for item := range other.m {
		if !s.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

// IsSubset 判断是否为另一个集合的子集.
func (s *HashSet[T]) IsSubset(other *HashSet[T]) bool {
	if s.Len() > other.Len() {
		return false
	}
	for item := range s.m {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// IsSuperset 判断是否为另一个集合的超集.
func (s *HashSet[T]) IsSuperset(other *HashSet[T]) bool {
	return other.IsSubset(s)
}

// Equal 判断两个集合是否相等.
func (s *HashSet[T]) Equal(other *HashSet[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	return s.IsSubset(other)
}

// IsDisjoint 判断两个集合是否不相交.
func (s *HashSet[T]) IsDisjoint(other *HashSet[T]) bool {
	// 遍历较小的集合
	smaller, larger := s, other
	if s.Len() > other.Len() {
		smaller, larger = other, s
	}

	for item := range smaller.m {
		if larger.Contains(item) {
			return false
		}
	}
	return true
}

// All 返回遍历所有元素的迭代器（顺序不确定）.
func (s *HashSet[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.m {
			if !yield(item) {
				return
			}
		}
	}
}
