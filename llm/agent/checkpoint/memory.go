package checkpoint

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore 内存实现的检查点存储，使用 sync.Map 存储.
type MemoryStore struct {
	m sync.Map
}

// NewMemoryStore 创建内存检查点存储.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Save 保存检查点.
func (s *MemoryStore) Save(_ context.Context, cp *AgentCheckpoint) error {
	s.m.Store(cp.ID, cp)
	return nil
}

// Load 加载检查点.
func (s *MemoryStore) Load(_ context.Context, id string) (*AgentCheckpoint, error) {
	v, ok := s.m.Load(id)
	if !ok {
		return nil, fmt.Errorf("checkpoint: 检查点不存在: %s", id)
	}
	return v.(*AgentCheckpoint), nil
}

// Delete 删除检查点.
func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.m.Delete(id)
	return nil
}
