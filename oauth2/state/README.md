# oauth2/state

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/oauth2/state"
```

## 简介

`oauth2/state` 提供 OAuth2 CSRF 防护用的 State 令牌存储实现。提供内存存储（`MemoryStore`）和 Redis 存储（`RedisStore`）两种实现，均实现 `oauth2.StateStore` 接口。State 令牌一次性使用，验证后自动销毁。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `MemoryStore` | 内存 State 存储，实现 `oauth2.StateStore` |
| `NewMemoryStore()` | 创建内存 State 存储 |
| `RedisStore` | Redis State 存储，实现 `oauth2.StateStore` |
| `NewRedisStore(client, opts...)` | 基于 `redis.Cmdable` 创建 Redis State 存储 |
| `WithPrefix(prefix)` | 设置 Redis Key 前缀 |
| `WithTTL(d)` | 设置 State 令牌过期时间 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/redis/go-redis/v9"

    "github.com/Tsukikage7/servex/v2/oauth2/state"
)

func main() {
    ctx := context.Background()

    // 内存存储（适合单机开发）
    memStore := state.NewMemoryStore()
    stateToken, _ := memStore.Generate(ctx)
    fmt.Println("生成 State:", stateToken)

    if err := memStore.Validate(ctx, stateToken); err != nil {
        fmt.Println("验证失败:", err)
    } else {
        fmt.Println("State 验证通过")
    }

    // 再次验证应失败（一次性使用）
    if err := memStore.Validate(ctx, stateToken); err != nil {
        fmt.Println("已消费，无法重复验证:", err)
    }

    // Redis 存储（适合多实例部署）
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    redisStore := state.NewRedisStore(rdb,
        state.WithPrefix("oauth2:state:"),
        state.WithTTL(10*60*1e9), // 10 分钟
    )

    token, _ := redisStore.Generate(ctx)
    fmt.Println("Redis State:", token)

    if err := redisStore.Validate(ctx, token); err == nil {
        fmt.Println("Redis State 验证通过")
    }
}
```
