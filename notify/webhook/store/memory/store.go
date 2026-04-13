// Package memory 提供基于内存的 webhook SubscriptionStore 实现，用于开发和测试.
package memory

import (
	"context"
	"sync"

	"github.com/Tsukikage7/servex/v2/notify/webhook"
)

// Store 内存实现的 SubscriptionStore，用于开发和测试.
type Store struct {
	mu   sync.RWMutex
	subs map[string]*webhook.Subscription
}

// NewStore 创建内存 SubscriptionStore 实例.
func NewStore() *Store {
	return &Store{subs: make(map[string]*webhook.Subscription)}
}

// Save 保存或更新订阅.
func (s *Store) Save(_ context.Context, sub *webhook.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[sub.ID] = sub
	return nil
}

// Delete 删除指定订阅.
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, id)
	return nil
}

// ListByEvent 按事件类型查询订阅列表.
func (s *Store) ListByEvent(_ context.Context, eventType string) ([]*webhook.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*webhook.Subscription
	for _, sub := range s.subs {
		if len(sub.Events) == 0 {
			result = append(result, sub)
			continue
		}
		for _, e := range sub.Events {
			if e == eventType {
				result = append(result, sub)
				break
			}
		}
	}
	return result, nil
}

// Get 按 ID 查询单个订阅.
func (s *Store) Get(_ context.Context, id string) (*webhook.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subs[id]
	if !ok {
		return nil, webhook.ErrNotFound
	}
	return sub, nil
}
