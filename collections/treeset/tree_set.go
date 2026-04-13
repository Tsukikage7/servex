// Package treeset 提供基于红黑树实现的有序集合.
// 注意：本包的数据结构非并发安全，如需在多 goroutine 中使用请自行加锁.
package treeset

import (
	"cmp"
	"iter"

	"github.com/Tsukikage7/servex/v2/collections/treemap"
)

// TreeSet 基于红黑树的有序集合.
// 特性:
//   - 元素按排序顺序存储
//   - Add/Remove/Contains 操作时间复杂度 O(log n)
//   - 支持自定义比较器
//   - 不允许重复元素
//
// 示例:
//
//	ts := treeset.NewOrdered[int]()
//	ts.Add(3, 1, 2)
//	ts.ToSlice() // [1, 2, 3]
type TreeSet[T any] struct {
	tm *treemap.TreeMap[T, struct{}]
}

// New 创建 TreeSet，需要提供比较器.
func New[T any](cmp treemap.Comparator[T]) *TreeSet[T] {
	return &TreeSet[T]{tm: treemap.New[T, struct{}](cmp)}
}

// NewOrdered 创建 TreeSet，使用内置类型的默认比较.
func NewOrdered[T cmp.Ordered]() *TreeSet[T] {
	return &TreeSet[T]{tm: treemap.NewOrdered[T, struct{}]()}
}

// FromSlice 从切片创建 TreeSet.
func FromSlice[T cmp.Ordered](items []T) *TreeSet[T] {
	s := NewOrdered[T]()
	s.Add(items...)
	return s
}

// Add 添加元素.
func (s *TreeSet[T]) Add(items ...T) {
	for _, item := range items {
		s.tm.Put(item, struct{}{})
	}
}

// Remove 移除元素.
func (s *TreeSet[T]) Remove(items ...T) {
	for _, item := range items {
		s.tm.Remove(item)
	}
}

// Contains 判断元素是否存在.
func (s *TreeSet[T]) Contains(item T) bool {
	return s.tm.ContainsKey(item)
}

// Len 返回元素数量.
func (s *TreeSet[T]) Len() int {
	return s.tm.Len()
}

// IsEmpty 判断是否为空.
func (s *TreeSet[T]) IsEmpty() bool {
	return s.tm.IsEmpty()
}

// Clear 清空所有元素.
func (s *TreeSet[T]) Clear() {
	s.tm.Clear()
}

// First 返回最小元素.
func (s *TreeSet[T]) First() (T, bool) {
	return s.tm.FirstKey()
}

// Last 返回最大元素.
func (s *TreeSet[T]) Last() (T, bool) {
	return s.tm.LastKey()
}

// ToSlice 返回所有元素（按排序顺序）.
func (s *TreeSet[T]) ToSlice() []T {
	return s.tm.Keys()
}

// Range 按顺序遍历所有元素.
// fn 返回 false 时停止遍历.
func (s *TreeSet[T]) Range(fn func(item T) bool) {
	s.tm.Range(func(key T, _ struct{}) bool {
		return fn(key)
	})
}

// Clone 克隆 TreeSet.
func (s *TreeSet[T]) Clone() *TreeSet[T] {
	return &TreeSet[T]{tm: s.tm.Clone()}
}

// Union 返回与另一个集合的并集.
func (s *TreeSet[T]) Union(other *TreeSet[T]) *TreeSet[T] {
	result := s.Clone()
	other.Range(func(item T) bool {
		result.Add(item)
		return true
	})
	return result
}

// Intersection 返回与另一个集合的交集.
func (s *TreeSet[T]) Intersection(other *TreeSet[T]) *TreeSet[T] {
	result := &TreeSet[T]{tm: treemap.New[T, struct{}](s.tm.Comparator())}

	// 遍历较小的集合
	smaller, larger := s, other
	if s.Len() > other.Len() {
		smaller, larger = other, s
	}

	smaller.Range(func(item T) bool {
		if larger.Contains(item) {
			result.Add(item)
		}
		return true
	})
	return result
}

// Difference 返回差集（s - other）.
func (s *TreeSet[T]) Difference(other *TreeSet[T]) *TreeSet[T] {
	result := s.Clone()
	other.Range(func(item T) bool {
		result.Remove(item)
		return true
	})
	return result
}

// IsSubset 判断是否为另一个集合的子集.
func (s *TreeSet[T]) IsSubset(other *TreeSet[T]) bool {
	if s.Len() > other.Len() {
		return false
	}
	isSubset := true
	s.Range(func(item T) bool {
		if !other.Contains(item) {
			isSubset = false
			return false
		}
		return true
	})
	return isSubset
}

// IsSuperset 判断是否为另一个集合的超集.
func (s *TreeSet[T]) IsSuperset(other *TreeSet[T]) bool {
	return other.IsSubset(s)
}

// Equal 判断两个集合是否相等.
func (s *TreeSet[T]) Equal(other *TreeSet[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	return s.IsSubset(other)
}

// All 返回按排序顺序遍历所有元素的迭代器.
func (s *TreeSet[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		s.tm.Range(func(key T, _ struct{}) bool {
			return yield(key)
		})
	}
}

// Backward 返回按排序逆序遍历所有元素的迭代器.
func (s *TreeSet[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		s.tm.RangeReverse(func(key T, _ struct{}) bool {
			return yield(key)
		})
	}
}
