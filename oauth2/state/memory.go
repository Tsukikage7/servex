// Package state 提供 OAuth2 state 参数的生成与验证实现.
package state

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultCleanupInterval = 5 * time.Minute

// MemoryStore 内存实现的 StateStore，用于开发和测试.
type MemoryStore struct {
	mu              sync.Mutex
	states          map[string]stateEntry
	ttl             time.Duration
	cleanupInterval time.Duration
	cancel          context.CancelFunc
	done            chan struct{}
	closeOnce       sync.Once
}

type stateEntry struct {
	expiresAt    time.Time
	codeVerifier string // PKCE code_verifier
}

// MemoryStoreOption 配置 MemoryStore 的选项函数.
type MemoryStoreOption func(*MemoryStore)

// WithCleanupInterval 设置过期 state 的清理间隔.
func WithCleanupInterval(d time.Duration) MemoryStoreOption {
	return func(s *MemoryStore) { s.cleanupInterval = d }
}

// WithTTL 设置 state 的过期时间.
func WithMemoryTTL(d time.Duration) MemoryStoreOption {
	return func(s *MemoryStore) { s.ttl = d }
}

// NewMemoryStore 创建基于内存的 StateStore 实例.
// 启动后台协程定期清理过期的 state.
func NewMemoryStore(opts ...MemoryStoreOption) *MemoryStore {
	s := &MemoryStore{
		states:          make(map[string]stateEntry),
		ttl:             10 * time.Minute,
		cleanupInterval: defaultCleanupInterval,
		done:            make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.cleanupLoop(ctx)
	return s
}

// Close 停止后台清理协程. 幂等安全.
func (s *MemoryStore) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		<-s.done
	})
}

// Generate 生成一个新的 state 参数并存储.
func (s *MemoryStore) Generate(_ context.Context) (string, error) {
	state := uuid.NewString()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state] = stateEntry{expiresAt: time.Now().Add(s.ttl)}
	return state, nil
}

// Validate 验证并消费一个 state 参数.
func (s *MemoryStore) Validate(_ context.Context, state string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.states[state]
	if !ok {
		return false, nil
	}
	delete(s.states, state) // 一次性消费
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}
	return true, nil
}

// SetCodeVerifier 将 PKCE code_verifier 与 state 关联.
func (s *MemoryStore) SetCodeVerifier(state, verifier string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.states[state]; ok {
		entry.codeVerifier = verifier
		s.states[state] = entry
	}
}

// ConsumeCodeVerifier 获取并删除与 state 关联的 code_verifier.
func (s *MemoryStore) ConsumeCodeVerifier(state string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.states[state]
	if !ok {
		return ""
	}
	verifier := entry.codeVerifier
	entry.codeVerifier = ""
	s.states[state] = entry
	return verifier
}

func (s *MemoryStore) cleanupLoop(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.removeExpired()
		}
	}
}

func (s *MemoryStore) removeExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for state, entry := range s.states {
		if now.After(entry.expiresAt) {
			delete(s.states, state)
		}
	}
}
