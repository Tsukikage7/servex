// Package redis 封装 go-redis/v9，提供统一的 Redis 操作接口.
//
// 特性：
//   - 完整的 Redis 数据类型操作（String/Hash/List/Set/Sorted Set）
//   - 脚本执行（Eval/EvalSha）
//   - Pipeline 批量操作
//   - Pub/Sub 发布订阅
//   - 配置校验与默认值
//
// 示例：
//
//	client, err := redis.NewClient(redis.DefaultConfig(), log)
//	if err != nil {
//	    panic(err)
//	}
//	defer client.Close()
//
//	_ = client.Set(ctx, "key", "value", time.Minute)
//	val, _ := client.Get(ctx, "key")
package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 错误定义.
var (
	// ErrNilConfig 配置为 nil 时返回.
	ErrNilConfig = errors.New("redis: config is nil")
	// ErrNilLogger 日志记录器为 nil 时返回.
	ErrNilLogger = errors.New("redis: logger is nil")
	// ErrEmptyAddr 地址为空时返回.
	ErrEmptyAddr = errors.New("redis: addr is empty")
)

// Config Redis 配置.
type Config struct {
	Addr          string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	Password      string        `json:"-" yaml:"password" mapstructure:"password"`
	DB            int           `json:"db" yaml:"db" mapstructure:"db"`
	MaxRetries    int           `json:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`
	PoolSize      int           `json:"pool_size" yaml:"pool_size" mapstructure:"pool_size"`
	MinIdleConns  int           `json:"min_idle_conns" yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
	DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout" mapstructure:"dial_timeout"`
	ReadTimeout   time.Duration `json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout"`
	EnableTracing bool          `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// DefaultConfig 返回默认 Redis 配置.
func DefaultConfig() *Config {
	return &Config{
		Addr:         "localhost:6379",
		DB:           0,
		MaxRetries:   3,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// Validate 校验配置.
func (c *Config) Validate() error {
	if c.Addr == "" {
		return ErrEmptyAddr
	}
	return nil
}

// ApplyDefaults 填充默认值.
func (c *Config) ApplyDefaults() {
	if c.Addr == "" {
		c.Addr = "localhost:6379"
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
	if c.MinIdleConns == 0 {
		c.MinIdleConns = 2
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
}

// Z 有序集合成员（封装 go-redis 的 Z 类型）.
type Z = goredis.Z

// Pipeline Redis 管道接口.
type Pipeline interface {
	Exec(ctx context.Context) error
	Set(ctx context.Context, key string, value any, expiration time.Duration) *goredis.StatusCmd
	Get(ctx context.Context, key string) *goredis.StringCmd
	Del(ctx context.Context, keys ...string) *goredis.IntCmd
}

// PubSub 发布订阅接口.
type PubSub interface {
	Channel(opts ...goredis.ChannelOption) <-chan *goredis.Message
	Close() error
}

// Client Redis 客户端接口.
type Client interface {
	// Ping 检查连接.
	Ping(ctx context.Context) error
	// Close 关闭连接.
	Close() error

	// String 操作
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) (int64, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// Hash 操作
	HSet(ctx context.Context, key string, values ...any) (int64, error)
	HGet(ctx context.Context, key, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) (int64, error)

	// List 操作
	LPush(ctx context.Context, key string, values ...any) (int64, error)
	RPush(ctx context.Context, key string, values ...any) (int64, error)
	LPop(ctx context.Context, key string) (string, error)
	RPop(ctx context.Context, key string) (string, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)

	// Set 操作
	SAdd(ctx context.Context, key string, members ...any) (int64, error)
	SMembers(ctx context.Context, key string) ([]string, error)
	SIsMember(ctx context.Context, key string, member any) (bool, error)
	SRem(ctx context.Context, key string, members ...any) (int64, error)

	// Sorted Set 操作
	ZAdd(ctx context.Context, key string, members ...Z) (int64, error)
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]Z, error)
	ZRank(ctx context.Context, key string, member string) (int64, error)
	ZScore(ctx context.Context, key string, member string) (float64, error)
	ZRem(ctx context.Context, key string, members ...any) (int64, error)
	ZCard(ctx context.Context, key string) (int64, error)

	// Script 操作
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...any) (any, error)
	ScriptLoad(ctx context.Context, script string) (string, error)

	// Pipeline 管道.
	PipelineExec(ctx context.Context, fn func(pipe goredis.Pipeliner) error) error

	// Pub/Sub 发布订阅.
	Subscribe(ctx context.Context, channels ...string) PubSub

	// Underlying 底层 go-redis 客户端.
	Underlying() *goredis.Client
}

// NewClient 创建 Redis 客户端.
func NewClient(config *Config, log logger.Logger) (Client, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if log == nil {
		return nil, ErrNilLogger
	}
	log = logger.WithComponent(log, "Redis")

	// 先填充默认值，再校验
	config.ApplyDefaults()

	if err := config.Validate(); err != nil {
		return nil, err
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		MaxRetries:   config.MaxRetries,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// 启用链路追踪
	if config.EnableTracing {
		if err := redisotel.InstrumentTracing(rdb); err != nil {
			_ = rdb.Close()
			return nil, err
		}
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}

	log.Info("连接成功", "addr", config.Addr, "db", config.DB)

	return &redisClient{
		client: rdb,
		log:    log,
	}, nil
}

// MustNewClient 创建 Redis 客户端，失败时 panic.
func MustNewClient(config *Config, log logger.Logger) Client {
	c, err := NewClient(config, log)
	if err != nil {
		panic(err)
	}
	return c
}
