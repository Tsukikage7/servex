# ratelimit

限流包，提供多种限流算法和中间件支持。

## 功能特性

- **多种限流算法**：令牌桶、滑动窗口、固定窗口
- **多层中间件**：Endpoint、HTTP、gRPC
- **分布式限流**：基于 Redis 等缓存实现
- **灵活的键提取**：支持基于 IP、路径、用户等维度限流

## 限流算法

### 令牌桶 (Token Bucket)

令牌以固定速率生成，请求消耗令牌才能通过。适合平滑突发流量。

```go
// 每秒生成 100 个令牌，桶容量 10
limiter := ratelimit.NewTokenBucket(100, 10)

if limiter.Allow(ctx) {
    // 处理请求
}
```

### 滑动窗口 (Sliding Window)

统计最近一个时间窗口内的请求数，超过阈值则拒绝。适合精确控制 QPS。

```go
// 每秒最多 100 个请求
limiter := ratelimit.NewSlidingWindow(100, time.Second)

if limiter.Allow(ctx) {
    // 处理请求
}
```

### 固定窗口 (Fixed Window)

将时间划分为固定窗口，每个窗口内限制请求数。实现简单但有边界突发问题。

```go
// 每秒最多 100 个请求
limiter := ratelimit.NewFixedWindow(100, time.Second)

if limiter.Allow(ctx) {
    // 处理请求
}
```

## Endpoint 中间件

与 `transport.Middleware` 集成，用于服务层限流。

```go
import (
    "github.com/Tsukikage7/servex/v2/ratelimit"
    "github.com/Tsukikage7/servex/v2/transport"
)

// 创建限流器
limiter := ratelimit.NewTokenBucket(100, 10)

// 方式1：直接拒绝（返回 ErrRateLimited）
endpoint = ratelimit.EndpointMiddleware(limiter)(endpoint)

// 方式2：阻塞等待（直到可用或超时）
endpoint = ratelimit.EndpointMiddlewareWithWait(limiter)(endpoint)

// 方式3：基于键的限流（如用户ID）
endpoint = ratelimit.KeyedEndpointMiddleware(
    func(ctx context.Context, req any) string {
        return req.(*Request).UserID
    },
    func(userID string) ratelimit.Limiter {
        return getUserLimiter(userID)
    },
)(endpoint)
```

## HTTP 中间件

用于 HTTP 服务器限流。

```go
import "github.com/Tsukikage7/servex/v2/ratelimit"

// 全局限流
limiter := ratelimit.NewTokenBucket(1000, 100)
handler = ratelimit.HTTPMiddleware(limiter)(handler)

// 基于 IP 限流
limiters := sync.Map{}
handler = ratelimit.KeyedHTTPMiddleware(
    ratelimit.IPKeyFunc(),
    func(ip string) ratelimit.Limiter {
        if l, ok := limiters.Load(ip); ok {
            return l.(ratelimit.Limiter)
        }
        l := ratelimit.NewTokenBucket(10, 5)
        limiters.Store(ip, l)
        return l
    },
)(handler)

// 基于路径限流
handler = ratelimit.KeyedHTTPMiddleware(
    ratelimit.PathKeyFunc(),
    getLimiterByPath,
)(handler)

// 组合键（IP + 路径）
handler = ratelimit.KeyedHTTPMiddleware(
    ratelimit.CompositeKeyFunc(
        ratelimit.IPKeyFunc(),
        ratelimit.PathKeyFunc(),
    ),
    getLimiter,
)(handler)
```

### HTTP 键提取函数

| 函数                    | 说明                                             |
| ----------------------- | ------------------------------------------------ |
| `IPKeyFunc()`           | 提取客户端 IP（支持 X-Forwarded-For、X-Real-IP） |
| `PathKeyFunc()`         | 提取请求路径                                     |
| `CompositeKeyFunc(...)` | 组合多个键提取函数                               |

## gRPC 拦截器

用于 gRPC 服务器限流。

```go
import (
    "github.com/Tsukikage7/servex/v2/ratelimit"
    "google.golang.org/grpc"
)

limiter := ratelimit.NewTokenBucket(1000, 100)

// 一元调用拦截器
server := grpc.NewServer(
    grpc.UnaryInterceptor(ratelimit.UnaryServerInterceptor(limiter)),
)

// 流式调用拦截器
server := grpc.NewServer(
    grpc.StreamInterceptor(ratelimit.StreamServerInterceptor(limiter)),
)

// 基于方法限流
server := grpc.NewServer(
    grpc.UnaryInterceptor(ratelimit.KeyedUnaryServerInterceptor(
        ratelimit.MethodKeyFunc(),
        getLimiterByMethod,
    )),
)

// 基于 metadata 限流（如用户ID）
server := grpc.NewServer(
    grpc.UnaryInterceptor(ratelimit.KeyedUnaryServerInterceptor(
        ratelimit.MetadataKeyFunc("user-id"),
        getLimiterByUser,
    )),
)
```

### gRPC 键提取函数

| 函数                        | 说明                   |
| --------------------------- | ---------------------- |
| `PeerKeyFunc()`             | 提取客户端地址         |
| `MethodKeyFunc()`           | 提取方法名             |
| `MetadataKeyFunc(key)`      | 提取指定 metadata 字段 |
| `CompositeGRPCKeyFunc(...)` | 组合多个键提取函数     |

## 分布式限流

基于 `cache` 包实现分布式限流，适用于多实例部署场景。

```go
import (
    "github.com/Tsukikage7/servex/v2/cache"
    "github.com/Tsukikage7/servex/v2/ratelimit"
)

// 创建 Redis 缓存
redisCache, _ := cache.New(&cache.Config{
    Type: cache.TypeRedis,
    Redis: cache.RedisConfig{
        Addr: "localhost:6379",
    },
})

// 创建分布式限流器
limiter, _ := ratelimit.NewDistributedLimiter(&ratelimit.DistributedConfig{
    Cache:  redisCache,
    Prefix: "api:ratelimit",
    Limit:  1000,           // 每秒最多 1000 个请求
    Window: time.Second,
})

// 使用方式与本地限流器相同
if limiter.Allow(ctx) {
    // 处理请求
}

// 基于键的分布式限流
keyedLimiter, _ := ratelimit.NewKeyedDistributedLimiter(&ratelimit.DistributedConfig{
    Cache:  redisCache,
    Prefix: "api:user:ratelimit",
    Limit:  100,
    Window: time.Second,
})

// 获取指定用户的限流器
userLimiter := keyedLimiter.GetLimiter(userID)
if userLimiter.Allow(ctx) {
    // 处理请求
}
```

## 配置方式

支持通过配置创建限流器：

```go
// 令牌桶配置
cfg := &ratelimit.Config{
    Algorithm: ratelimit.AlgorithmTokenBucket,
    Rate:      100,  // 每秒令牌数
    Capacity:  10,   // 桶容量
}

// 滑动窗口配置
cfg := &ratelimit.Config{
    Algorithm: ratelimit.AlgorithmSlidingWindow,
    Limit:     100,
    Window:    time.Second,
}

// 分布式限流配置
cfg := &ratelimit.Config{
    Algorithm: ratelimit.AlgorithmDistributed,
    Limit:     1000,
    Window:    time.Second,
    Prefix:    "api:ratelimit",
    Cache:     redisCache,
}

// 创建限流器
limiter, err := ratelimit.NewLimiter(cfg)
```

### 配置参数

| 参数        | 说明             | 适用算法                                  |
| ----------- | ---------------- | ----------------------------------------- |
| `Algorithm` | 算法类型         | 所有                                      |
| `Rate`      | 每秒令牌数       | token_bucket                              |
| `Capacity`  | 桶容量           | token_bucket                              |
| `Limit`     | 窗口内最大请求数 | sliding_window, fixed_window, distributed |
| `Window`    | 窗口大小         | sliding_window, fixed_window, distributed |
| `Prefix`    | 缓存键前缀       | distributed                               |
| `Cache`     | 缓存实例         | distributed                               |

### 算法类型常量

```go
const (
    AlgorithmTokenBucket   = "token_bucket"
    AlgorithmSlidingWindow = "sliding_window"
    AlgorithmFixedWindow   = "fixed_window"
    AlgorithmDistributed   = "distributed"
)
```

## 完整示例

### HTTP 服务限流

```go
package main

import (
    "net/http"
    "sync"
    "time"

    "github.com/Tsukikage7/servex/v2/ratelimit"
)

func main() {
    // 全局限流：1000 QPS
    globalLimiter := ratelimit.NewTokenBucket(1000, 100)

    // 每 IP 限流器缓存
    ipLimiters := sync.Map{}
    getIPLimiter := func(ip string) ratelimit.Limiter {
        if l, ok := ipLimiters.Load(ip); ok {
            return l.(ratelimit.Limiter)
        }
        l := ratelimit.NewSlidingWindow(100, time.Second)
        ipLimiters.Store(ip, l)
        return l
    }

    // 业务处理器
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })

    // 应用限流中间件（从外到内）
    // 1. 全局限流
    // 2. IP 限流
    finalHandler := ratelimit.HTTPMiddleware(globalLimiter)(
        ratelimit.KeyedHTTPMiddleware(ratelimit.IPKeyFunc(), getIPLimiter)(handler),
    )

    http.ListenAndServe(":8080", finalHandler)
}
```

### gRPC 服务限流

```go
package main

import (
    "time"

    "github.com/Tsukikage7/servex/v2/ratelimit"
    "google.golang.org/grpc"
)

func main() {
    // 全局限流
    globalLimiter := ratelimit.NewTokenBucket(1000, 100)

    // 方法级限流
    methodLimiters := map[string]ratelimit.Limiter{
        "/api.Service/HeavyMethod": ratelimit.NewSlidingWindow(10, time.Second),
        "/api.Service/LightMethod": ratelimit.NewSlidingWindow(1000, time.Second),
    }

    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            // 全局限流
            ratelimit.UnaryServerInterceptor(globalLimiter),
            // 方法级限流
            ratelimit.KeyedUnaryServerInterceptor(
                ratelimit.MethodKeyFunc(),
                func(method string) ratelimit.Limiter {
                    return methodLimiters[method]
                },
            ),
        ),
    )

    // 注册服务...
}
```

### Endpoint 层限流

```go
package main

import (
    "context"
    "time"

    "github.com/Tsukikage7/servex/v2/ratelimit"
    "github.com/Tsukikage7/servex/v2/transport"
)

func main() {
    // 业务 Endpoint
    var endpoint transport.Endpoint = func(ctx context.Context, req any) (any, error) {
        return "result", nil
    }

    // 应用限流中间件
    limiter := ratelimit.NewSlidingWindow(100, time.Second)
    endpoint = ratelimit.EndpointMiddleware(limiter)(endpoint)

    // 链式组合多个中间件
    endpoint = transport.Chain(
        ratelimit.EndpointMiddleware(limiter),
        // 其他中间件...
    )(endpoint)
}
```

## 错误处理

```go
import "github.com/Tsukikage7/servex/v2/ratelimit"

// 检查是否被限流
if errors.Is(err, ratelimit.ErrRateLimited) {
    // 返回 429 或 ResourceExhausted
}
```

### 预定义错误

| 错误               | 说明               |
| ------------------ | ------------------ |
| `ErrRateLimited`   | 请求被限流         |
| `ErrNilLimiter`    | 限流器为空         |
| `ErrInvalidConfig` | 配置无效           |
| `ErrNilCache`      | 分布式限流需要缓存 |

## 算法选择建议

| 场景          | 推荐算法   | 原因                                 |
| ------------- | ---------- | ------------------------------------ |
| API 网关      | 令牌桶     | 平滑突发流量，允许短时间内的流量突增 |
| 精确 QPS 控制 | 滑动窗口   | 精确统计每秒请求数                   |
| 简单场景      | 固定窗口   | 实现简单，资源消耗低                 |
| 多实例部署    | 分布式限流 | 跨实例统一限流                       |
