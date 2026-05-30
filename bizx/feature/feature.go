// Package feature 提供特性开关管理，支持百分比放量、白名单等策略.
package feature

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"sync"

	"github.com/redis/go-redis/v9"
)

// ErrFlagNotFound 特性开关不存在.
var ErrFlagNotFound = errors.New("feature: flag not found")

// Flag 特性开关.
type Flag struct {
	Name       string         `json:"name"`
	Enabled    bool           `json:"enabled"`
	Percentage int            `json:"percentage"`
	Users      []string       `json:"users,omitzero"`
	Groups     []string       `json:"groups,omitzero"`
	Metadata   map[string]any `json:"metadata,omitzero"`
}

// Manager 特性开关管理器.
type Manager interface {
	IsEnabled(ctx context.Context, name string, opts ...EvalOption) bool
	GetFlag(ctx context.Context, name string) (*Flag, error)
	SetFlag(ctx context.Context, flag *Flag) error
	DeleteFlag(ctx context.Context, name string) error
	ListFlags(ctx context.Context) ([]*Flag, error)
}

// EvalOption 评估选项.
type EvalOption func(*evalContext)

type evalContext struct {
	userID     string
	group      string
	attributes map[string]any
}

// WithUser 指定评估用户.
func WithUser(userID string) EvalOption {
	return func(ec *evalContext) {
		ec.userID = userID
	}
}

// WithGroup 指定评估分组.
func WithGroup(group string) EvalOption {
	return func(ec *evalContext) {
		ec.group = group
	}
}

// WithAttributes 指定附加属性.
func WithAttributes(attrs map[string]any) EvalOption {
	return func(ec *evalContext) {
		ec.attributes = attrs
	}
}

// Store 特性开关持久化接口.
type Store interface {
	Get(ctx context.Context, name string) (*Flag, error)
	Set(ctx context.Context, flag *Flag) error
	Delete(ctx context.Context, name string) error
	List(ctx context.Context) ([]*Flag, error)
}

// manager 特性开关管理器实现.
type manager struct {
	store Store
}

// NewManager 创建特性开关管理器.
func NewManager(store Store) Manager {
	return &manager{store: store}
}

// IsEnabled 判断特性是否对给定上下文启用.
func (m *manager) IsEnabled(ctx context.Context, name string, opts ...EvalOption) bool {
	flag, err := m.store.Get(ctx, name)
	if err != nil || flag == nil {
		return false
	}

	// 全局开关关闭
	if !flag.Enabled {
		return false
	}

	ec := &evalContext{}
	for _, opt := range opts {
		opt(ec)
	}

	// 用户白名单
	if ec.userID != "" {
		for _, u := range flag.Users {
			if u == ec.userID {
				return true
			}
		}
	}

	// 分组白名单
	if ec.group != "" {
		for _, g := range flag.Groups {
			if g == ec.group {
				return true
			}
		}
	}

	// 百分比放量
	if flag.Percentage > 0 && ec.userID != "" {
		h := fnv.New32a()
		_, _ = h.Write([]byte(flag.Name + ":" + ec.userID))
		bucket := int(h.Sum32() % 100)
		return bucket < flag.Percentage
	}

	// 如果没有白名单和百分比限制，且全局启用，则直接返回 true
	if len(flag.Users) == 0 && len(flag.Groups) == 0 && flag.Percentage == 0 {
		return true
	}

	return false
}

func (m *manager) GetFlag(ctx context.Context, name string) (*Flag, error) {
	return m.store.Get(ctx, name)
}

func (m *manager) SetFlag(ctx context.Context, flag *Flag) error {
	return m.store.Set(ctx, flag)
}

func (m *manager) DeleteFlag(ctx context.Context, name string) error {
	return m.store.Delete(ctx, name)
}

func (m *manager) ListFlags(ctx context.Context) ([]*Flag, error) {
	return m.store.List(ctx)
}

// --- Memory Store ---

type memoryStore struct {
	mu    sync.RWMutex
	flags map[string]*Flag
}

// NewMemoryStore 创建基于内存的特性开关存储用于测试.
func NewMemoryStore() Store {
	return &memoryStore{flags: make(map[string]*Flag)}
}

func (s *memoryStore) Get(_ context.Context, name string) (*Flag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flags[name]
	if !ok {
		return nil, ErrFlagNotFound
	}
	cp := *f
	return &cp, nil
}

func (s *memoryStore) Set(_ context.Context, flag *Flag) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *flag
	s.flags[flag.Name] = &cp
	return nil
}

func (s *memoryStore) Delete(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.flags[name]; !ok {
		return ErrFlagNotFound
	}
	delete(s.flags, name)
	return nil
}

func (s *memoryStore) List(_ context.Context) ([]*Flag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Flag, 0, len(s.flags))
	for _, f := range s.flags {
		cp := *f
		result = append(result, &cp)
	}
	return result, nil
}

// --- Redis Store ---

// StoreOption Redis Store 选项.
type StoreOption func(*redisStoreOptions)

type redisStoreOptions struct {
	prefix string
}

type redisStore struct {
	client redis.Cmdable
	opts   redisStoreOptions
}

// NewRedisStore 创建基于 Redis 的特性开关存储.
func NewRedisStore(client redis.Cmdable, opts ...StoreOption) Store {
	o := redisStoreOptions{prefix: "feature:"}
	for _, opt := range opts {
		opt(&o)
	}
	return &redisStore{client: client, opts: o}
}

func (s *redisStore) key(name string) string {
	return s.opts.prefix + name
}

func (s *redisStore) Get(ctx context.Context, name string) (*Flag, error) {
	data, err := s.client.Get(ctx, s.key(name)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrFlagNotFound
		}
		return nil, err
	}
	var flag Flag
	if err := json.Unmarshal(data, &flag); err != nil {
		return nil, err
	}
	return &flag, nil
}

func (s *redisStore) Set(ctx context.Context, flag *Flag) error {
	data, err := json.Marshal(flag)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(flag.Name), data, 0).Err()
}

func (s *redisStore) Delete(ctx context.Context, name string) error {
	result, err := s.client.Del(ctx, s.key(name)).Result()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrFlagNotFound
	}
	return nil
}

func (s *redisStore) List(ctx context.Context) ([]*Flag, error) {
	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = s.client.Scan(ctx, cursor, s.opts.prefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	result := make([]*Flag, 0, len(keys))
	for _, key := range keys {
		data, err := s.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var flag Flag
		if err := json.Unmarshal(data, &flag); err != nil {
			continue
		}
		result = append(result, &flag)
	}
	return result, nil
}
