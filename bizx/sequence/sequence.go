// Package sequence 提供业务序号生成器.
// 区别于 xutil/idgen 的全局唯一 ID，本包生成连续有意义的业务编号，
// 支持前缀、日期、补零、每日重置等功能.
// 基本用法:
//
//	store := sequence.NewMemoryStore()
//	seq := sequence.New(&sequence.Config{
//	    Name:       "order",
//	    Prefix:     "ORD-",
//	    DateFormat: "20060102",
//	    Padding:    4,
//	}, store)
//	id, _ := seq.Next(ctx) // "ORD-20260405-0001"
//	id, _ = seq.Next(ctx)  // "ORD-20260405-0002"
package sequence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrNilConfig 配置为空.
	ErrNilConfig = errors.New("sequence: config is nil")
	// ErrNilStore 存储为空.
	ErrNilStore = errors.New("sequence: store is nil")
	// ErrEmptyName 序列名为空.
	ErrEmptyName = errors.New("sequence: name is empty")
)

// Config 序列配置.
type Config struct {
	Name       string // 序列名（如 "order"）
	Prefix     string // 前缀（如 "ORD-"）
	DateFormat string // 日期格式（如 "20060102"），为空则不含日期
	Padding    int    // 序号补零位数，默认 4 → 0001
	Step       int64  // 步长，默认 1
	ResetDaily bool   // 每日重置，默认 false
}

// Store 序号持久化接口.
type Store interface {
	// GetAndIncrement 获取当前值并增加步长，返回增加前的值.
	GetAndIncrement(ctx context.Context, key string, step int64) (int64, error)
	// Reset 重置序号.
	Reset(ctx context.Context, key string) error
	// Current 获取当前值.
	Current(ctx context.Context, key string) (int64, error)
}

// Sequence 序号生成器接口.
type Sequence interface {
	// Next 生成下一个序号.
	Next(ctx context.Context) (string, error)
	// Current 获取当前序号（不递增）.
	Current(ctx context.Context) (string, error)
	// Reset 重置序号.
	Reset(ctx context.Context) error
}

// seq 序号生成器实现.
type seq struct {
	cfg   *Config
	store Store
}

// New 创建序号生成器.
func New(cfg *Config, store Store) Sequence {
	if cfg == nil {
		panic(ErrNilConfig)
	}
	if store == nil {
		panic(ErrNilStore)
	}
	if cfg.Name == "" {
		panic(ErrEmptyName)
	}
	if cfg.Padding <= 0 {
		cfg.Padding = 4
	}
	if cfg.Step <= 0 {
		cfg.Step = 1
	}
	return &seq{cfg: cfg, store: store}
}

func (s *seq) storeKey() string {
	key := "seq:" + s.cfg.Name
	if s.cfg.DateFormat != "" && s.cfg.ResetDaily {
		key += ":" + time.Now().Format(s.cfg.DateFormat)
	}
	return key
}

func (s *seq) format(num int64) string {
	result := s.cfg.Prefix
	if s.cfg.DateFormat != "" {
		result += time.Now().Format(s.cfg.DateFormat) + "-"
	}
	result += fmt.Sprintf("%0*d", s.cfg.Padding, num)
	return result
}

func (s *seq) Next(ctx context.Context) (string, error) {
	val, err := s.store.GetAndIncrement(ctx, s.storeKey(), s.cfg.Step)
	if err != nil {
		return "", err
	}
	return s.format(val + 1), nil
}

func (s *seq) Current(ctx context.Context) (string, error) {
	val, err := s.store.Current(ctx, s.storeKey())
	if err != nil {
		return "", err
	}
	if val == 0 {
		return s.format(0), nil
	}
	return s.format(val), nil
}

func (s *seq) Reset(ctx context.Context) error {
	return s.store.Reset(ctx, s.storeKey())
}

// ---- 内存 Store ----

// memoryStore 基于内存的序号存储.
type memoryStore struct {
	mu     sync.Mutex
	values map[string]int64
}

// NewMemoryStore 创建内存序号存储.
func NewMemoryStore() Store {
	return &memoryStore{values: make(map[string]int64)}
}

func (s *memoryStore) GetAndIncrement(_ context.Context, key string, step int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.values[key]
	s.values[key] = current + step
	return current, nil
}

func (s *memoryStore) Reset(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	return nil
}

func (s *memoryStore) Current(_ context.Context, key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[key], nil
}

// ---- Redis Store ----

// redisStore 基于 Redis 的序号存储.
type redisStore struct {
	client redis.Cmdable
}

// NewRedisStore 创建 Redis 序号存储.
func NewRedisStore(client redis.Cmdable) Store {
	return &redisStore{client: client}
}

func (s *redisStore) GetAndIncrement(ctx context.Context, key string, step int64) (int64, error) {
	// INCRBY 返回增加后的值，需要减去 step 得到增加前的值
	val, err := s.client.IncrBy(ctx, key, step).Result()
	if err != nil {
		return 0, err
	}
	return val - step, nil
}

func (s *redisStore) Reset(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *redisStore) Current(ctx context.Context, key string) (int64, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}
