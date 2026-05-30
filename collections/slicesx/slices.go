// Package slicesx 提供切片操作的工具函数.
// 注意：本包的函数非并发安全，如需在多 goroutine 中使用请自行加锁.
package slicesx

import (
	"fmt"
	"iter"
)

// Filter 过滤切片，返回满足条件的元素.
// 示例:
//
//	nums := []int{1, 2, 3, 4, 5}
//	evens := slices.Filter(nums, func(n int) bool { return n%2 == 0 })
//	// evens: [2, 4]
func Filter[T any](slice []T, fn func(T) bool) []T {
	result := make([]T, 0)
	for _, item := range slice {
		if fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map 对切片中的每个元素应用转换函数.
// 示例:
//
//	nums := []int{1, 2, 3}
//	strs := slices.Map(nums, strconv.Itoa)
//	// strs: ["1", "2", "3"]
func Map[T, R any](slice []T, fn func(T) R) []R {
	result := make([]R, len(slice))
	for i, item := range slice {
		result[i] = fn(item)
	}
	return result
}

// Reduce 将切片归约为单个值.
// 示例:
//
//	nums := []int{1, 2, 3, 4}
//	sum := slices.Reduce(nums, 0, func(acc, n int) int { return acc + n })
//	// sum: 10
func Reduce[T, R any](slice []T, initial R, fn func(R, T) R) R {
	result := initial
	for _, item := range slice {
		result = fn(result, item)
	}
	return result
}

// Unique 返回去重后的切片保持原顺序.
// 示例:
//
//	nums := []int{1, 2, 2, 3, 1}
//	unique := slices.Unique(nums)
//	// unique: [1, 2, 3]
func Unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{}, len(slice))
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// UniqueBy 根据键函数去重保持原顺序.
// 示例:
//
//	users := []User{{ID: 1, Name: "a"}, {ID: 1, Name: "b"}}
//	unique := slices.UniqueBy(users, func(u User) int { return u.ID })
//	// unique: [{ID: 1, Name: "a"}]
func UniqueBy[T any, K comparable](slice []T, keyFn func(T) K) []T {
	seen := make(map[K]struct{}, len(slice))
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		key := keyFn(item)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// GroupBy 根据键函数对切片进行分组.
// 示例:
//
//	nums := []int{1, 2, 3, 4, 5}
//	groups := slices.GroupBy(nums, func(n int) string {
//	    if n%2 == 0 { return "even" }
//	    return "odd"
//	})
//	// groups: {"odd": [1, 3, 5], "even": [2, 4]}
func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, item := range slice {
		key := keyFn(item)
		result[key] = append(result[key], item)
	}
	return result
}

// Chunk 将切片分割为指定大小的块.
// 示例:
//
//	nums := []int{1, 2, 3, 4, 5}
//	chunks := slices.Chunk(nums, 2)
//	// chunks: [[1, 2], [3, 4], [5]]
func Chunk[T any](slice []T, size int) [][]T {
	if size <= 0 {
		return nil
	}

	n := len(slice)
	chunks := make([][]T, 0, (n+size-1)/size)

	for i := 0; i < n; i += size {
		end := i + size
		if end > n {
			end = n
		}
		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

// Partition 将切片分为满足条件和不满足条件的两部分.
// 示例:
//
//	nums := []int{1, 2, 3, 4, 5}
//	evens, odds := slices.Partition(nums, func(n int) bool { return n%2 == 0 })
//	// evens: [2, 4], odds: [1, 3, 5]
func Partition[T any](slice []T, fn func(T) bool) (matched, unmatched []T) {
	matched = make([]T, 0)
	unmatched = make([]T, 0)
	for _, item := range slice {
		if fn(item) {
			matched = append(matched, item)
		} else {
			unmatched = append(unmatched, item)
		}
	}
	return
}

// Find 查找第一个满足条件的元素.
// 示例:
//
//	nums := []int{1, 2, 3, 4}
//	val, ok := slices.Find(nums, func(n int) bool { return n > 2 })
//	// val: 3, ok: true
func Find[T any](slice []T, fn func(T) bool) (T, bool) {
	for _, item := range slice {
		if fn(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// FindIndex 查找第一个满足条件的元素索引.
// 示例:
//
//	nums := []int{1, 2, 3, 4}
//	idx := slices.FindIndex(nums, func(n int) bool { return n > 2 })
//	// idx: 2
func FindIndex[T any](slice []T, fn func(T) bool) int {
	for i, item := range slice {
		if fn(item) {
			return i
		}
	}
	return -1
}

// Any 判断是否有任意元素满足条件.
// 示例:
//
//	nums := []int{1, 2, 3}
//	hasEven := slices.Any(nums, func(n int) bool { return n%2 == 0 })
//	// hasEven: true
func Any[T any](slice []T, fn func(T) bool) bool {
	for _, item := range slice {
		if fn(item) {
			return true
		}
	}
	return false
}

// All 判断是否所有元素都满足条件.
// 示例:
//
//	nums := []int{2, 4, 6}
//	allEven := slices.All(nums, func(n int) bool { return n%2 == 0 })
//	// allEven: true
func All[T any](slice []T, fn func(T) bool) bool {
	for _, item := range slice {
		if !fn(item) {
			return false
		}
	}
	return true
}

// None 判断是否没有元素满足条件.
// 示例:
//
//	nums := []int{1, 3, 5}
//	noEven := slices.None(nums, func(n int) bool { return n%2 == 0 })
//	// noEven: true
func None[T any](slice []T, fn func(T) bool) bool {
	return !Any(slice, fn)
}

// Count 统计满足条件的元素数量.
// 示例:
//
//	nums := []int{1, 2, 3, 4}
//	count := slices.Count(nums, func(n int) bool { return n%2 == 0 })
//	// count: 2
func Count[T any](slice []T, fn func(T) bool) int {
	count := 0
	for _, item := range slice {
		if fn(item) {
			count++
		}
	}
	return count
}

// Flatten 将二维切片扁平化为一维切片.
// 示例:
//
//	nested := [][]int{{1, 2}, {3, 4}, {5}}
//	flat := slices.Flatten(nested)
//	// flat: [1, 2, 3, 4, 5]
func Flatten[T any](slices [][]T) []T {
	total := 0
	for _, s := range slices {
		total += len(s)
	}

	result := make([]T, 0, total)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// Zip 将两个切片合并为键值对切片.
// 示例:
//
//	keys := []string{"a", "b", "c"}
//	vals := []int{1, 2, 3}
//	pairs := slices.Zip(keys, vals)
//	// pairs: [{"a", 1}, {"b", 2}, {"c", 3}]
func Zip[K, V any](keys []K, values []V) []Pair[K, V] {
	n := len(keys)
	if len(values) < n {
		n = len(values)
	}

	result := make([]Pair[K, V], n)
	for i := range n {
		result[i] = Pair[K, V]{First: keys[i], Second: values[i]}
	}
	return result
}

// Pair 键值对.
type Pair[K, V any] struct {
	First  K
	Second V
}

// ToMap 将键值对切片转换为 map.
// 示例:
//
//	pairs := []slices.Pair[string, int]{{"a", 1}, {"b", 2}}
//	m := slices.ToMap(pairs)
//	// m: {"a": 1, "b": 2}
func ToMap[K comparable, V any](pairs []Pair[K, V]) map[K]V {
	result := make(map[K]V, len(pairs))
	for _, p := range pairs {
		result[p.First] = p.Second
	}
	return result
}

// KeyBy 将切片转换为 map，使用键函数生成键.
// 示例:
//
//	users := []User{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
//	m := slices.KeyBy(users, func(u User) int { return u.ID })
//	// m: {1: {ID: 1, Name: "a"}, 2: {ID: 2, Name: "b"}}
func KeyBy[T any, K comparable](slice []T, keyFn func(T) K) map[K]T {
	result := make(map[K]T, len(slice))
	for _, item := range slice {
		result[keyFn(item)] = item
	}
	return result
}

// Compact 移除切片中的零值元素.
// 示例:
//
//	strs := []string{"a", "", "b", "", "c"}
//	compact := slices.Compact(strs)
//	// compact: ["a", "b", "c"]
func Compact[T comparable](slice []T) []T {
	var zero T
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if item != zero {
			result = append(result, item)
		}
	}
	return result
}

// First 返回第一个元素.
func First[T any](slice []T) (T, bool) {
	if len(slice) == 0 {
		var zero T
		return zero, false
	}
	return slice[0], true
}

// Last 返回最后一个元素.
func Last[T any](slice []T) (T, bool) {
	if len(slice) == 0 {
		var zero T
		return zero, false
	}
	return slice[len(slice)-1], true
}

// Take 返回前 n 个元素的副本.
func Take[T any](slice []T, n int) []T {
	if n <= 0 {
		return nil
	}
	if n > len(slice) {
		n = len(slice)
	}
	result := make([]T, n)
	copy(result, slice[:n])
	return result
}

// Skip 跳过前 n 个元素，返回剩余元素的副本.
func Skip[T any](slice []T, n int) []T {
	if n <= 0 {
		result := make([]T, len(slice))
		copy(result, slice)
		return result
	}
	if n >= len(slice) {
		return nil
	}
	result := make([]T, len(slice)-n)
	copy(result, slice[n:])
	return result
}

// TakeWhile 返回满足条件的前缀元素.
func TakeWhile[T any](slice []T, fn func(T) bool) []T {
	for i, item := range slice {
		if !fn(item) {
			return slice[:i]
		}
	}
	return slice
}

// SkipWhile 跳过满足条件的前缀元素.
func SkipWhile[T any](slice []T, fn func(T) bool) []T {
	for i, item := range slice {
		if !fn(item) {
			return slice[i:]
		}
	}
	return nil
}

// Contains 判断切片中是否包含目标元素.
// 示例:
//
//	nums := []int{1, 2, 3}
//	ok := Contains(nums, 2) // true
func Contains[T comparable](slice []T, target T) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

// ContainsAll 判断 src 是否包含 target 中的所有元素.
// 示例:
//
//	ContainsAll([]int{1, 2, 3}, []int{1, 2}) // true
func ContainsAll[T comparable](src, target []T) bool {
	set := make(map[T]struct{}, len(src))
	for _, item := range src {
		set[item] = struct{}{}
	}
	for _, item := range target {
		if _, ok := set[item]; !ok {
			return false
		}
	}
	return true
}

// ContainsAny 判断 src 是否包含 target 中的任意元素.
// 示例:
//
//	ContainsAny([]int{1, 2, 3}, []int{4, 2}) // true
func ContainsAny[T comparable](src, target []T) bool {
	set := make(map[T]struct{}, len(src))
	for _, item := range src {
		set[item] = struct{}{}
	}
	for _, item := range target {
		if _, ok := set[item]; ok {
			return true
		}
	}
	return false
}

// IntersectSet 返回两个切片的交集去重，保持 src 中的顺序.
// 示例:
//
//	IntersectSet([]int{1, 2, 3}, []int{2, 3, 4}) // [2, 3]
func IntersectSet[T comparable](src, dst []T) []T {
	set := make(map[T]struct{}, len(dst))
	for _, item := range dst {
		set[item] = struct{}{}
	}
	seen := make(map[T]struct{})
	result := make([]T, 0)
	for _, item := range src {
		if _, inDst := set[item]; inDst {
			if _, inSeen := seen[item]; !inSeen {
				seen[item] = struct{}{}
				result = append(result, item)
			}
		}
	}
	return result
}

// IntersectSetFunc 使用自定义相等函数返回两个切片的交集O(n*m).
func IntersectSetFunc[T any](src, dst []T, equal func(T, T) bool) []T {
	result := make([]T, 0)
outer:
	for _, s := range src {
		// 检查是否已加入结果
		for _, r := range result {
			if equal(s, r) {
				continue outer
			}
		}
		// 检查是否在 dst 中
		for _, d := range dst {
			if equal(s, d) {
				result = append(result, s)
				continue outer
			}
		}
	}
	return result
}

// UnionSet 返回两个切片的并集去重，保持顺序：先 src 后 dst 的新增部分.
// 示例:
//
//	UnionSet([]int{1, 2}, []int{2, 3}) // [1, 2, 3]
func UnionSet[T comparable](src, dst []T) []T {
	seen := make(map[T]struct{}, len(src)+len(dst))
	result := make([]T, 0, len(src)+len(dst))
	for _, item := range src {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	for _, item := range dst {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// UnionSetFunc 使用自定义相等函数返回两个切片的并集.
func UnionSetFunc[T any](src, dst []T, equal func(T, T) bool) []T {
	result := make([]T, 0, len(src)+len(dst))
	result = append(result, src...)
outer:
	for _, d := range dst {
		for _, r := range result {
			if equal(d, r) {
				continue outer
			}
		}
		result = append(result, d)
	}
	return result
}

// DiffSet 返回在 src 中存在但 dst 中不存在的元素差集，去重.
// 示例:
//
//	DiffSet([]int{1, 2, 3}, []int{2, 3}) // [1]
func DiffSet[T comparable](src, dst []T) []T {
	set := make(map[T]struct{}, len(dst))
	for _, item := range dst {
		set[item] = struct{}{}
	}
	seen := make(map[T]struct{})
	result := make([]T, 0)
	for _, item := range src {
		if _, inDst := set[item]; !inDst {
			if _, inSeen := seen[item]; !inSeen {
				seen[item] = struct{}{}
				result = append(result, item)
			}
		}
	}
	return result
}

// DiffSetFunc 使用自定义相等函数返回差集.
func DiffSetFunc[T any](src, dst []T, equal func(T, T) bool) []T {
	result := make([]T, 0)
outer:
	for _, s := range src {
		// 检查是否已在结果中
		for _, r := range result {
			if equal(s, r) {
				continue outer
			}
		}
		// 检查是否在 dst 中
		for _, d := range dst {
			if equal(s, d) {
				continue outer
			}
		}
		result = append(result, s)
	}
	return result
}

// SymmetricDiffSet 返回两个切片的对称差集只在其中一个集合中出现的元素，去重.
// 示例:
//
//	SymmetricDiffSet([]int{1, 2, 3}, []int{2, 3, 4}) // [1, 4]
func SymmetricDiffSet[T comparable](src, dst []T) []T {
	srcSet := make(map[T]struct{}, len(src))
	for _, item := range src {
		srcSet[item] = struct{}{}
	}
	dstSet := make(map[T]struct{}, len(dst))
	for _, item := range dst {
		dstSet[item] = struct{}{}
	}

	seen := make(map[T]struct{})
	result := make([]T, 0)

	for _, item := range src {
		if _, inDst := dstSet[item]; !inDst {
			if _, inSeen := seen[item]; !inSeen {
				seen[item] = struct{}{}
				result = append(result, item)
			}
		}
	}
	for _, item := range dst {
		if _, inSrc := srcSet[item]; !inSrc {
			if _, inSeen := seen[item]; !inSeen {
				seen[item] = struct{}{}
				result = append(result, item)
			}
		}
	}
	return result
}

// SymmetricDiffSetFunc 使用自定义相等函数返回对称差集.
func SymmetricDiffSetFunc[T any](src, dst []T, equal func(T, T) bool) []T {
	result := make([]T, 0)
	addUniq := func(item T) {
		for _, r := range result {
			if equal(item, r) {
				return
			}
		}
		result = append(result, item)
	}
	inDst := func(item T) bool {
		for _, d := range dst {
			if equal(item, d) {
				return true
			}
		}
		return false
	}
	inSrc := func(item T) bool {
		for _, s := range src {
			if equal(item, s) {
				return true
			}
		}
		return false
	}
	for _, s := range src {
		if !inDst(s) {
			addUniq(s)
		}
	}
	for _, d := range dst {
		if !inSrc(d) {
			addUniq(d)
		}
	}
	return result
}

// Sum 对数值切片求和.
// 示例:
//
//	Sum([]int{1, 2, 3}) // 6
func Sum[T Number](slice []T) T {
	var total T
	for _, item := range slice {
		total += item
	}
	return total
}

// Min 返回切片中的最小值；切片为空时返回零值和 false.
// 示例:
//
//	Min([]int{3, 1, 2}) // 1, true
func Min[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string
}](slice []T) (T, bool) {
	if len(slice) == 0 {
		var zero T
		return zero, false
	}
	m := slice[0]
	for _, item := range slice[1:] {
		if item < m {
			m = item
		}
	}
	return m, true
}

// Max 返回切片中的最大值；切片为空时返回零值和 false.
// 示例:
//
//	Max([]int{3, 1, 2}) // 3, true
func Max[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string
}](slice []T) (T, bool) {
	if len(slice) == 0 {
		var zero T
		return zero, false
	}
	m := slice[0]
	for _, item := range slice[1:] {
		if item > m {
			m = item
		}
	}
	return m, true
}

// IndexAll 返回目标元素在切片中所有出现的索引.
// 示例:
//
//	IndexAll([]int{1, 2, 1, 3}, 1) // [0, 2]
func IndexAll[T comparable](slice []T, target T) []int {
	result := make([]int, 0)
	for i, item := range slice {
		if item == target {
			result = append(result, i)
		}
	}
	return result
}

// LastIndex 返回目标元素最后一次出现的索引，不存在返回 -1.
// 示例:
//
//	LastIndex([]int{1, 2, 1, 3}, 1) // 2
func LastIndex[T comparable](slice []T, target T) int {
	for i := len(slice) - 1; i >= 0; i-- {
		if slice[i] == target {
			return i
		}
	}
	return -1
}

// Reverse 返回逆序的新切片.
// 示例:
//
//	Reverse([]int{1, 2, 3}) // [3, 2, 1]
func Reverse[T any](slice []T) []T {
	n := len(slice)
	result := make([]T, n)
	for i, item := range slice {
		result[n-1-i] = item
	}
	return result
}

// ReverseSelf 原地逆序切片.
// 示例:
//
//	s := []int{1, 2, 3}
//	ReverseSelf(s) // s: [3, 2, 1]
func ReverseSelf[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// Insert 在指定索引处插入元素，返回新切片；索引越界返回 error.
// 示例:
//
//	Insert([]int{1, 2, 3}, 1, 10) // [1, 10, 2, 3], nil
func Insert[T any](slice []T, index int, elem T) ([]T, error) {
	n := len(slice)
	if index < 0 || index > n {
		return nil, fmt.Errorf("slicesx: index %d out of range [0, %d]", index, n)
	}
	result := make([]T, n+1)
	copy(result, slice[:index])
	result[index] = elem
	copy(result[index+1:], slice[index:])
	return result, nil
}

// Delete 删除指定索引处的元素，返回新切片；索引越界返回 error.
// 示例:
//
//	Delete([]int{1, 2, 3}, 1) // [1, 3], nil
func Delete[T any](slice []T, index int) ([]T, error) {
	n := len(slice)
	if index < 0 || index >= n {
		return nil, fmt.Errorf("slicesx: index %d out of range [0, %d)", index, n)
	}
	result := make([]T, n-1)
	copy(result, slice[:index])
	copy(result[index:], slice[index+1:])
	return result, nil
}

// IterAll 返回遍历切片的索引-值迭代器.
// 等价于标准库 slices.All.
func IterAll[T any](slice []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range slice {
			if !yield(i, v) {
				return
			}
		}
	}
}

// IterValues 返回遍历切片值的迭代器.
// 等价于标准库 slices.Values.
func IterValues[T any](slice []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range slice {
			if !yield(v) {
				return
			}
		}
	}
}

// IterBackward 返回逆序遍历切片的索引-值迭代器.
// 等价于标准库 slices.Backward.
func IterBackward[T any](slice []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := len(slice) - 1; i >= 0; i-- {
			if !yield(i, slice[i]) {
				return
			}
		}
	}
}

// Collect 从迭代器收集元素到切片.
// 等价于标准库 slices.Collect.
func Collect[T any](seq iter.Seq[T]) []T {
	var result []T
	for v := range seq {
		result = append(result, v)
	}
	return result
}
