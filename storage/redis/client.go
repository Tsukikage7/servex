package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// redisClient 封装 go-redis 客户端.
type redisClient struct {
	client *goredis.Client
	log    logger.Logger
}

func (c *redisClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *redisClient) Close() error {
	return c.client.Close()
}

// ============================================================================
// String 操作
// ============================================================================

func (c *redisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *redisClient) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *redisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Del(ctx, keys...).Result()
}

func (c *redisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Exists(ctx, keys...).Result()
}

func (c *redisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return c.client.Expire(ctx, key, expiration).Result()
}

func (c *redisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

func (c *redisClient) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *redisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

// ============================================================================
// Hash 操作
// ============================================================================

func (c *redisClient) HSet(ctx context.Context, key string, values ...any) (int64, error) {
	return c.client.HSet(ctx, key, values...).Result()
}

func (c *redisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

func (c *redisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

func (c *redisClient) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return c.client.HDel(ctx, key, fields...).Result()
}

// ============================================================================
// List 操作
// ============================================================================

func (c *redisClient) LPush(ctx context.Context, key string, values ...any) (int64, error) {
	return c.client.LPush(ctx, key, values...).Result()
}

func (c *redisClient) RPush(ctx context.Context, key string, values ...any) (int64, error) {
	return c.client.RPush(ctx, key, values...).Result()
}

func (c *redisClient) LPop(ctx context.Context, key string) (string, error) {
	return c.client.LPop(ctx, key).Result()
}

func (c *redisClient) RPop(ctx context.Context, key string) (string, error) {
	return c.client.RPop(ctx, key).Result()
}

func (c *redisClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.LRange(ctx, key, start, stop).Result()
}

func (c *redisClient) LLen(ctx context.Context, key string) (int64, error) {
	return c.client.LLen(ctx, key).Result()
}

// ============================================================================
// Set 操作
// ============================================================================

func (c *redisClient) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	return c.client.SAdd(ctx, key, members...).Result()
}

func (c *redisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.client.SMembers(ctx, key).Result()
}

func (c *redisClient) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return c.client.SIsMember(ctx, key, member).Result()
}

func (c *redisClient) SRem(ctx context.Context, key string, members ...any) (int64, error) {
	return c.client.SRem(ctx, key, members...).Result()
}

// ============================================================================
// Sorted Set 操作
// ============================================================================

func (c *redisClient) ZAdd(ctx context.Context, key string, members ...Z) (int64, error) {
	return c.client.ZAdd(ctx, key, members...).Result()
}

func (c *redisClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.ZRange(ctx, key, start, stop).Result()
}

func (c *redisClient) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]Z, error) {
	return c.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (c *redisClient) ZRank(ctx context.Context, key string, member string) (int64, error) {
	return c.client.ZRank(ctx, key, member).Result()
}

func (c *redisClient) ZScore(ctx context.Context, key string, member string) (float64, error) {
	return c.client.ZScore(ctx, key, member).Result()
}

func (c *redisClient) ZRem(ctx context.Context, key string, members ...any) (int64, error) {
	return c.client.ZRem(ctx, key, members...).Result()
}

func (c *redisClient) ZCard(ctx context.Context, key string) (int64, error) {
	return c.client.ZCard(ctx, key).Result()
}

// ============================================================================
// Script 操作
// ============================================================================

func (c *redisClient) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return c.client.Eval(ctx, script, keys, args...).Result()
}

func (c *redisClient) EvalSha(ctx context.Context, sha1 string, keys []string, args ...any) (any, error) {
	return c.client.EvalSha(ctx, sha1, keys, args...).Result()
}

func (c *redisClient) ScriptLoad(ctx context.Context, script string) (string, error) {
	return c.client.ScriptLoad(ctx, script).Result()
}

// ============================================================================
// Pipeline 操作
// ============================================================================

func (c *redisClient) PipelineExec(ctx context.Context, fn func(pipe goredis.Pipeliner) error) error {
	pipe := c.client.Pipeline()
	if err := fn(pipe); err != nil {
		return err
	}
	_, err := pipe.Exec(ctx)
	return err
}

// ============================================================================
// Pub/Sub 操作
// ============================================================================

func (c *redisClient) Subscribe(ctx context.Context, channels ...string) PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// ============================================================================
// 底层访问
// ============================================================================

// Underlying 返回底层 go-redis 客户端.
func (c *redisClient) Underlying() *goredis.Client {
	return c.client
}
