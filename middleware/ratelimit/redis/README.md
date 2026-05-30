# middleware/ratelimit/redis

`github.com/Tsukikage7/servex/v2/middleware/ratelimit/redis` — 基于 Redis 的分布式**令牌桶**限流器。使用 Lua 原子脚本实现 refill + take，保证多实例并发下的正确性。

## 为什么用令牌桶

相比固定/滑动窗口，令牌桶能平滑突发流量：桶内令牌累积到 `capacity`，偶尔的爆发也能被吸收，只要长期 QPS 不超过 `rate`。

## 核心类型

- `RedisTokenBucket` — 分布式令牌桶限流器，实现 `middleware/ratelimit.Limiter` 的 `Allow`/`AllowN`/`Wait`/`WaitN` 方法
- `NewTokenBucket(client, rate, capacity, opts...)` — 构造函数
  - `client` 接受 `redis.UniversalClient`（兼容单实例 / Cluster / Ring）
  - `rate` 每秒生成的令牌数
  - `capacity` 桶容量
- `WithKeyFunc(fn)` — 从 `context` 动态抽取限流维度（例如 user_id / api_key / ip），不设置则使用单一默认 key
- `WithTTL(d)` — 自定义 Redis key 过期时间（默认 `2 * capacity / rate` 秒，最少 1 秒）

## 失败策略

**fail-closed**：Redis 错误、`client` 为 nil、`n` 超过桶容量均返回 `false` / 拒绝。`middleware/ratelimit` 包的 `DistributedLimiter` 默认同样 fail-closed；确实需要后端错误放行时显式设置 `FailOpen: true`。

## Lua 脚本

脚本位于 `token_bucket.go` 中常量 `luaScript`。每次 `AllowN` 触发一次 `EVALSHA`（未命中则 fallback `EVAL`）：

```
KEYS[1] = bucket key        -- HASH,字段 tokens / last
ARGV[1] = rate              -- tokens/sec
ARGV[2] = capacity          -- 桶容量
ARGV[3] = now (ms)          -- 当前时间戳,毫秒
ARGV[4] = n                 -- 请求消耗的令牌数
ARGV[5] = ttl (sec)         -- HASH 过期时间
return 1 表示允许,0 表示拒绝
```

## 使用示例

```go
import (
    "github.com/redis/go-redis/v9"
    ratelimitredis "github.com/Tsukikage7/servex/v2/middleware/ratelimit/redis"
)

rdb := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{"localhost:6379"}})

// 每用户每秒 10 个请求,突发 20 个.
tb := ratelimitredis.NewTokenBucket(rdb, 10, 20,
    ratelimitredis.WithKeyFunc(func(ctx context.Context) string {
        uid, _ := ctx.Value(ctxKeyUser).(string)
        return "rl:user:" + uid
    }),
    ratelimitredis.WithTTL(1*time.Minute),
)

if !tb.Allow(ctx) {
    http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

## 测试

`token_bucket_test.go` 使用包内注入点 `setScriptRunnerForTest` 替换 Lua 执行器为纯内存实现（`fakeBucketStore`），行为与 Redis + Lua 一致，无需 miniredis / 真实 Redis 实例。

## 依赖

- `github.com/redis/go-redis/v9`（已在 servex `go.mod` 中）
