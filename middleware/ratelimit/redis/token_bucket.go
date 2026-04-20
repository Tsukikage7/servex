// Package redis 提供基于 Redis 的分布式令牌桶限流器.
// 使用 Lua 原子脚本实现 refill + take,适合多实例共享限流状态的场景.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// luaScript 令牌桶 Lua 脚本,保证 refill + take 的原子性.
//
// KEYS[1]   : bucket key (HASH,字段 tokens / last)
// ARGV[1]   : rate (tokens/sec)
// ARGV[2]   : capacity (桶容量)
// ARGV[3]   : now (毫秒时间戳)
// ARGV[4]   : n (本次请求消耗的令牌数)
// ARGV[5]   : ttl (秒,HASH 过期时间)
//
// 返回 1 表示允许,0 表示拒绝.
const luaScript = `
local data = redis.call('HMGET', KEYS[1], 'tokens', 'last')
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local n = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])
local tokens = tonumber(data[1]) or capacity
local last = tonumber(data[2]) or now
local delta = math.max(0, (now - last) / 1000 * rate)
tokens = math.min(capacity, tokens + delta)
local allowed = 0
if tokens >= n then
  tokens = tokens - n
  allowed = 1
end
redis.call('HMSET', KEYS[1], 'tokens', tokens, 'last', now)
redis.call('EXPIRE', KEYS[1], ttl)
return allowed
`

// Script 预编译的脚本单例.
var script = redis.NewScript(luaScript)

// scriptRunner 抽象了 Lua 脚本执行,便于在测试中用 fake 替换.
// 生产代码永远使用 realScriptRunner 把调用委托给 redis.UniversalClient.
type scriptRunner func(ctx context.Context, keys []string, args ...any) (int64, error)

// RedisTokenBucket 基于 Redis 的分布式令牌桶限流器.
// 实现 middleware/ratelimit 包 Limiter 接口(Allow/AllowN/Wait/WaitN).
type RedisTokenBucket struct {
	client redis.UniversalClient
	run    scriptRunner  // 注入点:生产为 realScriptRunner,测试可替换
	nowFn  func() int64  // 注入点:当前毫秒时间戳,默认 time.Now().UnixMilli(),测试可冻结

	keyFn func(ctx context.Context) string

	rate     float64 // tokens/sec
	capacity float64
	ttl      time.Duration
}

// Option 构造选项.
type Option func(*RedisTokenBucket)

// WithKeyFunc 设置基于 context 的动态 key 函数.
// 默认 key 为 "ratelimit:tokenbucket:default".
// 若 fn 返回空串则 fallback 到默认 key.
func WithKeyFunc(fn func(context.Context) string) Option {
	return func(tb *RedisTokenBucket) {
		if fn != nil {
			tb.keyFn = fn
		}
	}
}

// WithTTL 覆盖 bucket 过期时间.
// 默认值为 2 * capacity / rate(单位:秒),保证桶充盈后再加一倍时间的保底窗口.
// 传 <=0 时忽略.
func WithTTL(d time.Duration) Option {
	return func(tb *RedisTokenBucket) {
		if d > 0 {
			tb.ttl = d
		}
	}
}

// defaultKey 默认 key.
const defaultKey = "ratelimit:tokenbucket:default"

// NewTokenBucket 创建 Redis 令牌桶限流器.
//
// 参数校验:
//   - client 不能为 nil,否则返回的限流器所有 AllowN 调用都会返回 false.
//   - rate / capacity 必须 > 0,否则回退为 1.
func NewTokenBucket(client redis.UniversalClient, rate, capacity float64, opts ...Option) *RedisTokenBucket {
	if rate <= 0 {
		rate = 1
	}
	if capacity <= 0 {
		capacity = 1
	}
	tb := &RedisTokenBucket{
		client:   client,
		rate:     rate,
		capacity: capacity,
		// 默认 TTL: 2 * capacity/rate 秒,最小 1 秒.
		ttl: defaultTTL(rate, capacity),
		keyFn: func(context.Context) string {
			return defaultKey
		},
		nowFn: func() int64 { return time.Now().UnixMilli() },
	}
	tb.run = realScriptRunner(client)
	for _, opt := range opts {
		opt(tb)
	}
	return tb
}

// realScriptRunner 生产用脚本执行器:调用 redis.UniversalClient 执行 Lua 脚本.
func realScriptRunner(client redis.UniversalClient) scriptRunner {
	if client == nil {
		return nil
	}
	return func(ctx context.Context, keys []string, args ...any) (int64, error) {
		return script.Run(ctx, client, keys, args...).Int64()
	}
}

// setScriptRunnerForTest 供包内测试注入自定义脚本执行器.
// 不是公开 API.
func (tb *RedisTokenBucket) setScriptRunnerForTest(run scriptRunner) {
	tb.run = run
}

// setNowFnForTest 供包内测试注入冻结时钟.
// 不是公开 API.
func (tb *RedisTokenBucket) setNowFnForTest(fn func() int64) {
	if fn != nil {
		tb.nowFn = fn
	}
}

// defaultTTL 计算默认 TTL.
func defaultTTL(rate, capacity float64) time.Duration {
	seconds := 2 * capacity / rate
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds * float64(time.Second))
}

// Allow 检查是否允许 1 个请求通过.
func (tb *RedisTokenBucket) Allow(ctx context.Context) bool {
	return tb.AllowN(ctx, 1)
}

// AllowN 检查是否允许 n 个请求通过.
// 任一错误条件(client nil / n<=0 / Redis 错误) 都返回 false,避免"失败开放"绕过限流.
func (tb *RedisTokenBucket) AllowN(ctx context.Context, n int) bool {
	if tb == nil || tb.run == nil {
		return false
	}
	if n <= 0 {
		return false
	}
	if float64(n) > tb.capacity {
		// 请求量大于桶容量永远不会通过.
		return false
	}
	key := tb.keyFn(ctx)
	if key == "" {
		key = defaultKey
	}
	ttlSec := int64(tb.ttl / time.Second)
	if ttlSec < 1 {
		ttlSec = 1
	}
	nowMs := tb.nowFn()
	res, err := tb.run(ctx, []string{key},
		tb.rate, tb.capacity, nowMs, n, ttlSec,
	)
	if err != nil {
		return false
	}
	return res == 1
}

// Wait 阻塞等待直到允许 1 个请求通过.
func (tb *RedisTokenBucket) Wait(ctx context.Context) error {
	return tb.WaitN(ctx, 1)
}

// WaitN 阻塞等待直到允许 n 个请求通过.
//
// 返回 error 表示 context 取消/超时,或超出桶容量的永远不可满足条件.
func (tb *RedisTokenBucket) WaitN(ctx context.Context, n int) error {
	if tb == nil || tb.run == nil {
		return fmt.Errorf("ratelimit/redis: client is nil")
	}
	if n <= 0 {
		return fmt.Errorf("ratelimit/redis: n must be > 0, got %d", n)
	}
	if float64(n) > tb.capacity {
		return fmt.Errorf("ratelimit/redis: n (%d) exceeds capacity (%v)", n, tb.capacity)
	}
	for {
		if tb.AllowN(ctx, n) {
			return nil
		}
		// 预估下一次有 n 个令牌需要的时间.
		waitTime := time.Duration(float64(n) / tb.rate * float64(time.Second))
		if waitTime < 10*time.Millisecond {
			waitTime = 10 * time.Millisecond
		}
		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
