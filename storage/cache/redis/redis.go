// Package redis provides a Redis-backed cache implementation.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/storage/cache"
)

func init() {
	cache.Register(cache.TypeRedis, NewRedisCache)
}

// redisCache Redis 缓存实现.
type redisCache struct {
	client *goredis.Client
	config *cache.Config
	logger logger.Logger
}

// NewRedisCache 创建 Redis 缓存.
func NewRedisCache(config *cache.Config, log logger.Logger) (cache.Cache, error) {
	if config == nil {
		return nil, cache.ErrNilConfig
	}
	log = logger.WithComponent(log, "Cache")

	if config.Addr == "" {
		return nil, cache.ErrEmptyAddr
	}

	config.ApplyDefaults()

	client := goredis.NewClient(&goredis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		DialTimeout:  config.Timeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		MaxRetries:   config.MaxRetries,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.With(
			logger.String("addr", config.Addr),
			logger.Err(err),
		).Error("Redis 连接失败")
		return nil, cache.ErrConnect
	}

	log.With(
		logger.String("addr", config.Addr),
		logger.Int("db", config.DB),
	).Debug("Redis 连接成功")

	return &redisCache{
		client: client,
		config: config,
		logger: log,
	}, nil
}

// Set 设置键值对.
func (r *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := r.serialize(value)
	if err != nil {
		return err
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("SET 操作失败")
		return err
	}
	return nil
}

// Get 获取值.
func (r *redisCache) Get(ctx context.Context, key string) (string, error) {
	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", cache.ErrNotFound
		}
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("GET 操作失败")
		return "", err
	}
	return result, nil
}

// Del 删除键.
func (r *redisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		r.logger.With(
			logger.Any("keys", keys),
			logger.Err(err),
		).Error("DEL 操作失败")
		return err
	}
	return nil
}

// Exists 检查键是否存在.
func (r *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("EXISTS 操作失败")
		return false, err
	}
	return result > 0, nil
}

// SetNX 仅当键不存在时设置.
func (r *redisCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	data, err := r.serialize(value)
	if err != nil {
		return false, err
	}

	result, err := r.client.SetNX(ctx, key, data, ttl).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("SETNX 操作失败")
		return false, err
	}
	return result, nil
}

// Increment 递增.
func (r *redisCache) Increment(ctx context.Context, key string) (int64, error) {
	result, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("INCR 操作失败")
		return 0, err
	}
	return result, nil
}

// IncrementBy 增加指定值.
func (r *redisCache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	result, err := r.client.IncrBy(ctx, key, value).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("INCRBY 操作失败")
		return 0, err
	}
	return result, nil
}

// Decrement 递减.
func (r *redisCache) Decrement(ctx context.Context, key string) (int64, error) {
	result, err := r.client.Decr(ctx, key).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("DECR 操作失败")
		return 0, err
	}
	return result, nil
}

// Expire 设置过期时间.
func (r *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := r.client.Expire(ctx, key, ttl).Err(); err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("EXPIRE 操作失败")
		return err
	}
	return nil
}

// TTL 获取剩余过期时间.
func (r *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("TTL 操作失败")
		return 0, err
	}
	return result, nil
}

// TryLock 尝试获取分布式锁.
func (r *redisCache) TryLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	result, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("获取锁失败")
		return false, err
	}
	return result, nil
}

// Unlock 释放分布式锁.
func (r *redisCache) Unlock(ctx context.Context, key string, value string) error {
	// Lua 脚本：只有当锁的值匹配时才删除
	script := goredis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, r.client, []string{key}, value).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("释放锁失败")
		return err
	}

	val, ok := result.(int64)
	if !ok || val == 0 {
		r.logger.With(
			logger.String("key", key),
			logger.String("reason", "值不匹配或已过期"),
		).Warn("释放锁已跳过")
		return cache.ErrLockNotHeld
	}

	return nil
}

// ExtendLock 原子地验证锁持有者并延长过期时间.
func (r *redisCache) ExtendLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	// Lua 脚本：原子地检查持有者并延长过期时间
	script := goredis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, r.client, []string{key}, value, ttl.Milliseconds()).Result()
	if err != nil {
		r.logger.With(
			logger.String("key", key),
			logger.Err(err),
		).Error("延长锁失败")
		return false, err
	}

	val, ok := result.(int64)
	if !ok {
		return false, nil
	}
	return val == 1, nil
}

// MGet 批量获取.
func (r *redisCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}

	results, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		r.logger.With(
			logger.Any("keys", keys),
			logger.Err(err),
		).Error("MGET 操作失败")
		return nil, err
	}

	values := make([]string, len(results))
	for i, v := range results {
		if v != nil {
			values[i] = v.(string)
		}
	}
	return values, nil
}

// MSet 批量设置.
func (r *redisCache) MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error {
	if len(pairs) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()

	for key, value := range pairs {
		data, err := r.serialize(value)
		if err != nil {
			return err
		}
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		r.logger.With(logger.Err(err)).Error("MSET 操作失败")
		return err
	}
	return nil
}

// Ping 测试连接.
func (r *redisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close 关闭连接.
func (r *redisCache) Close() error {
	if err := r.client.Close(); err != nil {
		r.logger.With(logger.Err(err)).Error("Redis 关闭失败")
		return err
	}
	r.logger.Debug("Redis 连接已关闭")
	return nil
}

// Client 返回底层客户端.
func (r *redisCache) Client() any {
	return r.client
}

// serialize 序列化值.
func (r *redisCache) serialize(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return "", cache.ErrSerialize
		}
		return string(data), nil
	}
}
