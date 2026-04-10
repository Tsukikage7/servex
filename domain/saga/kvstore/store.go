// Package sagakvstore 提供基于 KV 接口的 saga.Store 实现.
package sagakvstore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Tsukikage7/servex/domain/saga"
	"github.com/Tsukikage7/servex/storage/cache"
)

// stateDTO 用于序列化的状态结构.
type stateDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Status      saga.SagaStatus `json:"status"`
	CurrentStep int             `json:"current_step"`
	StepResults []stepResultDTO `json:"step_results"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitzero"`
	Data        map[string]any  `json:"data,omitzero"`
}

// stepResultDTO 用于序列化的步骤结果结构.
type stepResultDTO struct {
	StepName string          `json:"step_name"`
	Status   saga.StepStatus `json:"status"`
	Error    string          `json:"error,omitempty"`
	Duration int64           `json:"duration"`
}

// toDTO 将 State 转换为 DTO.
func toDTO(state *saga.State) *stateDTO {
	dto := &stateDTO{
		ID:          state.ID,
		Name:        state.Name,
		Status:      state.Status,
		CurrentStep: state.CurrentStep,
		StepResults: make([]stepResultDTO, len(state.StepResults)),
		Error:       state.Error,
		StartedAt:   state.StartedAt,
		CompletedAt: state.CompletedAt,
		Data:        state.Data,
	}

	for i, r := range state.StepResults {
		dto.StepResults[i] = stepResultDTO{
			StepName: r.StepName,
			Status:   r.Status,
			Duration: r.Duration,
		}
		if r.Error != nil {
			dto.StepResults[i].Error = r.Error.Error()
		}
	}

	return dto
}

// fromDTO 将 DTO 转换为 State.
func fromDTO(dto *stateDTO) *saga.State {
	state := &saga.State{
		ID:          dto.ID,
		Name:        dto.Name,
		Status:      dto.Status,
		CurrentStep: dto.CurrentStep,
		StepResults: make([]saga.StepResult, len(dto.StepResults)),
		Error:       dto.Error,
		StartedAt:   dto.StartedAt,
		CompletedAt: dto.CompletedAt,
		Data:        dto.Data,
	}

	for i, r := range dto.StepResults {
		state.StepResults[i] = saga.StepResult{
			StepName: r.StepName,
			Status:   r.Status,
			Duration: r.Duration,
		}
		// 注意：序列化后 error 信息会丢失类型，只保留消息
	}

	return state
}

// Store 基于 saga.KV 接口的 Saga 状态存储.
// 适用于分布式部署场景.
type Store struct {
	kv         saga.KV
	keyPrefix  string
	defaultTTL time.Duration
}

// Option KV 存储配置选项.
type Option func(*Store)

// WithKeyPrefix 设置键前缀.
func WithKeyPrefix(prefix string) Option {
	return func(s *Store) {
		s.keyPrefix = prefix
	}
}

// WithTTL 设置默认 TTL.
func WithTTL(ttl time.Duration) Option {
	return func(s *Store) {
		s.defaultTTL = ttl
	}
}

// NewStore 创建 KV 存储.
// kv: KV 存储实现（可用 CacheKV 适配 cache.Cache）
func NewStore(kv saga.KV, opts ...Option) *Store {
	s := &Store{
		kv:         kv,
		keyPrefix:  "saga:",
		defaultTTL: 24 * time.Hour, // 默认24小时
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Save 保存 Saga 状态.
func (s *Store) Save(ctx context.Context, state *saga.State) error {
	dto := toDTO(state)
	data, err := json.Marshal(dto)
	if err != nil {
		return err
	}

	key := s.keyPrefix + state.ID
	return s.kv.Set(ctx, key, string(data), s.defaultTTL)
}

// Get 获取 Saga 状态.
func (s *Store) Get(ctx context.Context, id string) (*saga.State, error) {
	key := s.keyPrefix + id
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		return nil, saga.ErrSagaNotFound
	}
	if data == "" {
		return nil, saga.ErrSagaNotFound
	}

	var dto stateDTO
	if err := json.Unmarshal([]byte(data), &dto); err != nil {
		return nil, err
	}

	return fromDTO(&dto), nil
}

// Delete 删除 Saga 状态.
func (s *Store) Delete(ctx context.Context, id string) error {
	key := s.keyPrefix + id
	return s.kv.Del(ctx, key)
}

// List 列出指定状态的 Saga.
// 注意: KV 实现不支持高效的条件查询，返回空列表.
// 建议在生产环境使用专门的索引或数据库来支持列表查询.
func (s *Store) List(ctx context.Context, status saga.SagaStatus, limit int) ([]*saga.State, error) {
	// KV 不支持高效的条件查询，返回空列表
	// 如果需要此功能，建议使用数据库存储或维护额外的索引
	return nil, nil
}

// cacheKV 是 cache.Cache 到 saga.KV 的适配器.
type cacheKV struct {
	cache cache.Cache
}

// CacheKV 将 cache.Cache 适配为 saga.KV 接口.
// 示例:
//
//	redisCache, _ := cache.New(&cache.Config{Type: "redis", ...})
//	kv := sagakvstore.CacheKV(redisCache)
//	store := sagakvstore.NewStore(kv)
func CacheKV(c cache.Cache) saga.KV {
	return &cacheKV{cache: c}
}

func (c *cacheKV) Get(ctx context.Context, key string) (string, error) {
	return c.cache.Get(ctx, key)
}

func (c *cacheKV) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.cache.Set(ctx, key, value, ttl)
}

func (c *cacheKV) Del(ctx context.Context, keys ...string) error {
	return c.cache.Del(ctx, keys...)
}
