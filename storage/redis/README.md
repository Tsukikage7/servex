# storage/redis

封装 [go-redis/v9](https://github.com/redis/go-redis)，提供完整的 Redis 操作接口，包含 String、Hash、List、Set、Sorted Set、Script、Pipeline 和 Pub/Sub。

## 特性

- 完整的 Redis 数据类型操作（String / Hash / List / Set / Sorted Set）
- Lua 脚本执行（`Eval` / `EvalSha` / `ScriptLoad`）
- Pipeline 批量操作（`PipelineExec`）
- Pub/Sub 发布订阅
- 配置校验与自动填充默认值
- 可选 OpenTelemetry 链路追踪（`EnableTracing`）
- `Underlying()` 暴露底层 `*goredis.Client`，兼容第三方库

## 快速开始

```go
import (
    "github.com/Tsukikage7/servex/storage/redis"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

log, _ := logger.NewLogger()
client, err := redis.NewClient(redis.DefaultConfig(), log)
if err != nil {
    panic(err)
}
defer client.Close()

// String 操作
_ = client.Set(ctx, "key", "value", time.Minute)
val, _ := client.Get(ctx, "key")

// 失败时 panic（适合 main 函数）
client = redis.MustNewClient(redis.DefaultConfig(), log)
```

## 配置

```go
cfg := &redis.Config{
    Addr:          "localhost:6379", // 必填
    Password:      "",
    DB:            0,
    MaxRetries:    3,
    PoolSize:      10,
    MinIdleConns:  2,
    DialTimeout:   5 * time.Second,
    ReadTimeout:   3 * time.Second,
    WriteTimeout:  3 * time.Second,
    EnableTracing: true,
}

// 或使用默认配置（localhost:6379，连接池 10，超时 3s）
cfg = redis.DefaultConfig()
```

## 常用操作

```go
// Hash
client.HSet(ctx, "user:1", "name", "Alice", "age", 30)
name, _ := client.HGet(ctx, "user:1", "name")
all, _  := client.HGetAll(ctx, "user:1")

// List（队列）
client.RPush(ctx, "queue", "task1", "task2")
task, _ := client.LPop(ctx, "queue")

// Set
client.SAdd(ctx, "tags", "go", "redis")
ok, _ := client.SIsMember(ctx, "tags", "go") // true

// Sorted Set（排行榜）
client.ZAdd(ctx, "leaderboard", goredis.Z{Score: 100, Member: "alice"})
top, _ := client.ZRangeWithScores(ctx, "leaderboard", 0, 9)
```

## Pipeline 批量操作

```go
err := client.PipelineExec(ctx, func(pipe goredis.Pipeliner) error {
    pipe.Set(ctx, "k1", "v1", time.Minute)
    pipe.Set(ctx, "k2", "v2", time.Minute)
    pipe.Incr(ctx, "counter")
    return nil
})
```

## Pub/Sub

```go
// 订阅
sub := client.Subscribe(ctx, "notifications")
defer sub.Close()
for msg := range sub.Channel() {
    fmt.Println(msg.Channel, msg.Payload)
}
```

## 核心接口

`redis.Client` — 完整操作接口，`NewClient(config, log)` 创建，`MustNewClient` 失败时 panic。
