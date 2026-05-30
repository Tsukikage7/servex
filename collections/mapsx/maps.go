// Package mapsx 提供 map 操作的工具函数.
// 注意：本包的函数非并发安全，如需在多 goroutine 中使用请自行加锁.
package mapsx

import "iter"

// Keys 返回 map 的所有键.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	keys := maps.Keys(m)
//	// keys: ["a", "b"] (顺序不确定)
func Keys[K comparable, V any](m map[K]V) []K {
	result := make([]K, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// Values 返回 map 的所有值.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	values := maps.Values(m)
//	// values: [1, 2] (顺序不确定)
func Values[K comparable, V any](m map[K]V) []V {
	result := make([]V, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

// Entries 返回 map 的所有键值对.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	entries := maps.Entries(m)
func Entries[K comparable, V any](m map[K]V) []Entry[K, V] {
	result := make([]Entry[K, V], 0, len(m))
	for k, v := range m {
		result = append(result, Entry[K, V]{Key: k, Value: v})
	}
	return result
}

// Entry 键值对.
type Entry[K comparable, V any] struct {
	Key   K
	Value V
}

// FromEntries 从键值对创建 map.
// 示例:
//
//	entries := []maps.Entry[string, int]{{"a", 1}, {"b", 2}}
//	m := maps.FromEntries(entries)
//	// m: {"a": 1, "b": 2}
func FromEntries[K comparable, V any](entries []Entry[K, V]) map[K]V {
	result := make(map[K]V, len(entries))
	for _, e := range entries {
		result[e.Key] = e.Value
	}
	return result
}

// Merge 合并多个 map后面的覆盖前面的.
// 示例:
//
//	m1 := map[string]int{"a": 1}
//	m2 := map[string]int{"b": 2, "a": 10}
//	merged := maps.Merge(m1, m2)
//	// merged: {"a": 10, "b": 2}
func Merge[K comparable, V any](maps ...map[K]V) map[K]V {
	total := 0
	for _, m := range maps {
		total += len(m)
	}

	result := make(map[K]V, total)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// Filter 过滤 map，返回满足条件的键值对.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2, "c": 3}
//	filtered := maps.Filter(m, func(k string, v int) bool { return v > 1 })
//	// filtered: {"b": 2, "c": 3}
func Filter[K comparable, V any](m map[K]V, fn func(K, V) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}

// FilterKeys 过滤 map，只保留指定的键.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2, "c": 3}
//	filtered := maps.FilterKeys(m, "a", "c")
//	// filtered: {"a": 1, "c": 3}
func FilterKeys[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	result := make(map[K]V, len(keys))
	for _, k := range keys {
		if v, ok := m[k]; ok {
			result[k] = v
		}
	}
	return result
}

// OmitKeys 过滤 map，排除指定的键.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2, "c": 3}
//	filtered := maps.OmitKeys(m, "a")
//	// filtered: {"b": 2, "c": 3}
func OmitKeys[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	exclude := make(map[K]struct{}, len(keys))
	for _, k := range keys {
		exclude[k] = struct{}{}
	}

	result := make(map[K]V)
	for k, v := range m {
		if _, ok := exclude[k]; !ok {
			result[k] = v
		}
	}
	return result
}

// MapKeys 转换 map 的键.
// 示例:
//
//	m := map[int]string{1: "a", 2: "b"}
//	mapped := maps.MapKeys(m, strconv.Itoa)
//	// mapped: {"1": "a", "2": "b"}
func MapKeys[K comparable, V any, NK comparable](m map[K]V, fn func(K) NK) map[NK]V {
	result := make(map[NK]V, len(m))
	for k, v := range m {
		result[fn(k)] = v
	}
	return result
}

// MapValues 转换 map 的值.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	mapped := maps.MapValues(m, func(v int) int { return v * 10 })
//	// mapped: {"a": 10, "b": 20}
func MapValues[K comparable, V, NV any](m map[K]V, fn func(V) NV) map[K]NV {
	result := make(map[K]NV, len(m))
	for k, v := range m {
		result[k] = fn(v)
	}
	return result
}

// Invert 反转 map键值互换.
// 如果有重复值，后面的会覆盖前面的.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	inverted := maps.Invert(m)
//	// inverted: {1: "a", 2: "b"}
func Invert[K comparable, V comparable](m map[K]V) map[V]K {
	result := make(map[V]K, len(m))
	for k, v := range m {
		result[v] = k
	}
	return result
}

// Clone 复制 map浅拷贝.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	cloned := maps.Clone(m)
func Clone[K comparable, V any](m map[K]V) map[K]V {
	result := make(map[K]V, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Equal 比较两个 map 是否相等.
// 示例:
//
//	m1 := map[string]int{"a": 1, "b": 2}
//	m2 := map[string]int{"a": 1, "b": 2}
//	equal := maps.Equal(m1, m2) // true
func Equal[K, V comparable](m1, m2 map[K]V) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		if v2, ok := m2[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}

// EqualBy 使用自定义比较函数比较两个 map.
func EqualBy[K comparable, V any](m1, m2 map[K]V, eq func(V, V) bool) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		if v2, ok := m2[k]; !ok || !eq(v1, v2) {
			return false
		}
	}
	return true
}

// GetOrDefault 获取值，不存在则返回默认值.
// 示例:
//
//	m := map[string]int{"a": 1}
//	val := maps.GetOrDefault(m, "b", 100)
//	// val: 100
func GetOrDefault[K comparable, V any](m map[K]V, key K, defaultVal V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultVal
}

// GetOrPut 获取值，不存在则设置并返回默认值.
// 示例:
//
//	m := map[string]int{"a": 1}
//	val := maps.GetOrPut(m, "b", 100)
//	// val: 100, m: {"a": 1, "b": 100}
func GetOrPut[K comparable, V any](m map[K]V, key K, defaultVal V) V {
	if v, ok := m[key]; ok {
		return v
	}
	m[key] = defaultVal
	return defaultVal
}

// GetOrCompute 获取值，不存在则计算并设置.
// 示例:
//
//	m := map[string]int{"a": 1}
//	val := maps.GetOrCompute(m, "b", func() int { return 100 })
//	// val: 100, m: {"a": 1, "b": 100}
func GetOrCompute[K comparable, V any](m map[K]V, key K, compute func() V) V {
	if v, ok := m[key]; ok {
		return v
	}
	v := compute()
	m[key] = v
	return v
}

// ForEach 遍历 map.
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	maps.ForEach(m, func(k string, v int) {
//	    fmt.Printf("%s: %d\n", k, v)
//	})
func ForEach[K comparable, V any](m map[K]V, fn func(K, V)) {
	for k, v := range m {
		fn(k, v)
	}
}

// Any 判断是否有任意键值对满足条件.
func Any[K comparable, V any](m map[K]V, fn func(K, V) bool) bool {
	for k, v := range m {
		if fn(k, v) {
			return true
		}
	}
	return false
}

// All 判断是否所有键值对都满足条件.
func All[K comparable, V any](m map[K]V, fn func(K, V) bool) bool {
	for k, v := range m {
		if !fn(k, v) {
			return false
		}
	}
	return true
}

// None 判断是否没有键值对满足条件.
func None[K comparable, V any](m map[K]V, fn func(K, V) bool) bool {
	return !Any(m, fn)
}

// Count 统计满足条件的键值对数量.
func Count[K comparable, V any](m map[K]V, fn func(K, V) bool) int {
	count := 0
	for k, v := range m {
		if fn(k, v) {
			count++
		}
	}
	return count
}

// ContainsKey 判断是否包含指定键.
func ContainsKey[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

// ContainsValue 判断是否包含指定值.
func ContainsValue[K, V comparable](m map[K]V, value V) bool {
	for _, v := range m {
		if v == value {
			return true
		}
	}
	return false
}

// FindKey 查找满足条件的第一个键.
func FindKey[K comparable, V any](m map[K]V, fn func(K, V) bool) (K, bool) {
	for k, v := range m {
		if fn(k, v) {
			return k, true
		}
	}
	var zero K
	return zero, false
}

// Diff 返回两个 map 的差异.
// added: 在 m2 中有但 m1 中没有的键
// removed: 在 m1 中有但 m2 中没有的键
// changed: 两个 map 都有但值不同的键
func Diff[K, V comparable](m1, m2 map[K]V) (added, removed, changed []K) {
	added = make([]K, 0)
	removed = make([]K, 0)
	changed = make([]K, 0)

	for k, v1 := range m1 {
		if v2, ok := m2[k]; ok {
			if v1 != v2 {
				changed = append(changed, k)
			}
		} else {
			removed = append(removed, k)
		}
	}

	for k := range m2 {
		if _, ok := m1[k]; !ok {
			added = append(added, k)
		}
	}

	return
}

// KeysValues 同时返回 map 的键切片和值切片，顺序保持一致即 keys[i] 对应 values[i].
// 示例:
//
//	m := map[string]int{"a": 1, "b": 2}
//	keys, values := KeysValues(m)
//	// keys 与 values 同序对应
func KeysValues[K comparable, V any](m map[K]V) ([]K, []V) {
	keys := make([]K, 0, len(m))
	values := make([]V, 0, len(m))
	for k, v := range m {
		keys = append(keys, k)
		values = append(values, v)
	}
	return keys, values
}

// MergeFunc 合并多个 map，遇到重复键时调用 mergeFunc 解决冲突.
// mergeFunc(v1, v2) 接收已有值 v1 和新值 v2，返回最终存储的值.
// 示例:
//
//	m1 := map[string]int{"a": 1}
//	m2 := map[string]int{"a": 2, "b": 3}
//	merged := MergeFunc(func(v1, v2 int) int { return v1 + v2 }, m1, m2)
//	// merged: {"a": 3, "b": 3}
func MergeFunc[K comparable, V any](mergeFunc func(v1, v2 V) V, maps ...map[K]V) map[K]V {
	result := make(map[K]V)
	for _, m := range maps {
		for k, v := range m {
			if existing, ok := result[k]; ok {
				result[k] = mergeFunc(existing, v)
			} else {
				result[k] = v
			}
		}
	}
	return result
}

// IterAll 返回遍历 map 所有键值对的迭代器顺序不确定.
// 等价于标准库 maps.All.
func IterAll[K comparable, V any](m map[K]V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	}
}

// IterKeys 返回遍历 map 所有键的迭代器顺序不确定.
// 等价于标准库 maps.Keys.
func IterKeys[K comparable, V any](m map[K]V) iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// IterValues 返回遍历 map 所有值的迭代器顺序不确定.
// 等价于标准库 maps.Values.
func IterValues[K comparable, V any](m map[K]V) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range m {
			if !yield(v) {
				return
			}
		}
	}
}

// Collect 从键值对迭代器收集到 map.
// 等价于标准库 maps.Collect.
func Collect[K comparable, V any](seq iter.Seq2[K, V]) map[K]V {
	result := make(map[K]V)
	for k, v := range seq {
		result[k] = v
	}
	return result
}
