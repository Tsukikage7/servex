package saga

import (
	"context"
	"time"
)

// Store Saga 状态存储接口.
type Store interface {
	// Save 保存 Saga 状态.
	Save(ctx context.Context, state *State) error

	// Get 获取 Saga 状态.
	Get(ctx context.Context, id string) (*State, error)

	// Delete 删除 Saga 状态.
	Delete(ctx context.Context, id string) error

	// List 列出指定状态的 Saga.
	List(ctx context.Context, status SagaStatus, limit int) ([]*State, error)
}

// KV Saga 状态存储所需的键值存储接口.
// 这是 saga 包的最小依赖接口.
// 可以用 cache.Cache、Redis 客户端或其他存储实现.
type KV interface {
	// Get 获取键的值.
	Get(ctx context.Context, key string) (string, error)

	// Set 设置键值对.
	Set(ctx context.Context, key string, value string, ttl time.Duration) error

	// Del 删除键.
	Del(ctx context.Context, keys ...string) error
}

// nopStore 空存储，不保存任何状态.
// 适用于不需要持久化状态的场景.
type nopStore struct{}

// newNopStore 创建空存储（内部使用）.
func newNopStore() *nopStore {
	return &nopStore{}
}

// Save 保存状态（空实现）.
func (s *nopStore) Save(ctx context.Context, state *State) error {
	return nil
}

// Get 获取状态（始终返回未找到）.
func (s *nopStore) Get(ctx context.Context, id string) (*State, error) {
	return nil, ErrSagaNotFound
}

// Delete 删除状态（空实现）.
func (s *nopStore) Delete(ctx context.Context, id string) error {
	return nil
}

// List 列出状态（返回空列表）.
func (s *nopStore) List(ctx context.Context, status SagaStatus, limit int) ([]*State, error) {
	return nil, nil
}
