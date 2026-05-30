package syncx

import "sync"

// Map 泛型并发安全 Map，包装 sync.Map 提供类型安全.
// 零值可用，无需初始化.
type Map[K comparable, V any] struct {
	m sync.Map
}

// Load 返回 key 对应的值，不存在时返回零值和 false.
func (m *Map[K, V]) Load(key K) (V, bool) {
	val, ok := m.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Store 存储 key-value 键值对.
func (m *Map[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

// LoadOrStore 返回 key 对应的已有值；不存在则存储并返回 value.
func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	actual, loaded := m.m.LoadOrStore(key, value)
	return actual.(V), loaded
}

// LoadAndDelete 删除 key 并返回其之前的值，不存在时返回零值和 false.
func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	val, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Delete 删除 key 对应的键值对.
func (m *Map[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Range 遍历所有键值对，fn 返回 false 时停止遍历.
func (m *Map[K, V]) Range(fn func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		return fn(key.(K), value.(V))
	})
}

// LoadOrStoreFunc 懒初始化：key 存在时直接返回已有值loaded=true；
// 不存在则调用 fn 创建后存入并返回loaded=false.
// 返回的 err 来自 fn；若 fn 返回 error，则不存储.
//
// 注意：在并发场景下，fn 可能被多个 goroutine 同时调用，但最终只有一个值会被存储.
// 如需保证 fn 仅执行一次，调用方应自行使用 sync.Once 或其他机制.
func (m *Map[K, V]) LoadOrStoreFunc(key K, fn func() (V, error)) (actual V, loaded bool, err error) {
	if val, ok := m.Load(key); ok {
		return val, true, nil
	}
	v, err := fn()
	if err != nil {
		return v, false, err
	}
	// 使用 LoadOrStore 处理并发场景：若两个 goroutine 同时到达此处，
	// 只有第一个会成功存储，另一个拿到已存储的值
	actual, loaded = m.LoadOrStore(key, v)
	return actual, loaded, nil
}
