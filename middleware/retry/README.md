# retry

提供重试机制，支持 Endpoint、HTTP 和 gRPC 中间件/拦截器。

## 功能特性

- **多种退避策略**：固定、指数、线性退避
- **可配置重试条件**：根据错误类型判断是否重试
- **多层中间件**：Endpoint、HTTP Client、gRPC Client
- **上下文支持**：支持超时和取消

## API

### 配置

```go
type Config struct {
    MaxAttempts int           // 最大重试次数（默认 3）
    Delay       time.Duration // 重试间隔（默认 100ms）
    Backoff     BackoffFunc   // 退避策略
    Retryable   RetryableFunc // 重试判断函数
}
```

### 退避策略

| 函数                 | 说明                           |
| -------------------- | ------------------------------ |
| `FixedBackoff`       | 固定间隔，每次重试等待相同时间 |
| `ExponentialBackoff` | 指数退避，等待时间翻倍增长     |
| `LinearBackoff`      | 线性退避，等待时间线性增长     |

```go
// 示例：100ms 基础延迟
// FixedBackoff:      100ms, 100ms, 100ms, 100ms
// ExponentialBackoff: 100ms, 200ms, 400ms, 800ms
// LinearBackoff:     100ms, 200ms, 300ms, 400ms
```

### 重试判断函数

| 函数                           | 说明                 |
| ------------------------------ | -------------------- |
| `AlwaysRetry`                  | 总是重试（默认）     |
| `NeverRetry`                   | 从不重试             |
| `RetryableCodesFunc(codes...)` | 根据 gRPC 状态码判断 |

### HTTP 重试判断

| 函数                     | 说明                   |
| ------------------------ | ---------------------- |
| `DefaultHTTPRetryable`   | 重试网络错误、5xx、429 |
| `RetryOn5xx`             | 仅重试 5xx 错误        |
| `RetryOnConnectionError` | 仅重试连接错误         |

### gRPC 默认重试状态码

默认对以下状态码进行重试：

- `codes.Unavailable` - 服务不可用
- `codes.ResourceExhausted` - 资源耗尽（如限流）
- `codes.Aborted` - 操作中止
- `codes.DeadlineExceeded` - 超时

## 使用示例

### Endpoint 层重试

```go
package main

import (
    "context"
    stderrors "errors"
    "time"

    servexerr "github.com/Tsukikage7/servex/v2/errors"
    "github.com/Tsukikage7/servex/v2/retry"
    "github.com/Tsukikage7/servex/v2/transport"
)

func main() {
    // 定义业务 Endpoint
    var paymentEndpoint transport.Endpoint = func(ctx context.Context, req any) (any, error) {
        // 支付处理逻辑
        return processPayment(req)
    }

    // 自定义重试条件：仅重试临时错误
    cfg := &retry.Config{
        MaxAttempts: 3,
        Delay:       200 * time.Millisecond,
        Backoff:     retry.ExponentialBackoff,
        Retryable: func(err error) bool {
            // 不重试业务错误
            var bizErr *servexerr.Error
            if stderrors.As(err, &bizErr) {
                return false
            }
            return true
        },
    }

    // 应用重试中间件
    paymentEndpoint = retry.EndpointMiddleware(cfg)(paymentEndpoint)

    // 链式组合多个中间件
    paymentEndpoint = transport.Chain(
        retry.EndpointMiddleware(cfg),
        // 其他中间件...
    )(paymentEndpoint)
}
```

### HTTP 客户端重试

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/Tsukikage7/servex/v2/retry"
)

func main() {
    // 创建可重试的 HTTP 客户端
    cfg := &retry.Config{
        MaxAttempts: 5,
        Delay:       100 * time.Millisecond,
        Backoff:     retry.ExponentialBackoff,
    }

    client := retry.NewHTTPClient(http.DefaultClient, cfg)

    // 发送请求
    req, _ := http.NewRequest("GET", "http://api.example.com/data", nil)
    resp, err := client.Do(req)
    if err != nil {
        // 处理错误
    }
    defer resp.Body.Close()

    // 自定义重试条件
    customClient := retry.NewHTTPClient(http.DefaultClient, cfg).
        WithRetryable(func(resp *http.Response, err error) bool {
            // 仅重试特定错误
            if err != nil {
                return true
            }
            // 重试 502, 503, 504
            return resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504
        })

    resp, err = customClient.Do(req)
}
```

### gRPC 客户端重试

```go
package main

import (
    "time"

    "github.com/Tsukikage7/servex/v2/retry"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // 方式1：使用配置
    cfg := &retry.Config{
        MaxAttempts: 3,
        Delay:       100 * time.Millisecond,
        Backoff:     retry.ExponentialBackoff,
        Retryable:   retry.RetryableCodesFunc(codes.Unavailable, codes.ResourceExhausted),
    }

    conn, _ := grpc.Dial("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(retry.UnaryClientInterceptor(cfg)),
        grpc.WithStreamInterceptor(retry.StreamClientInterceptor(cfg)),
    )
    defer conn.Close()

    // 方式2：使用 Retrier
    retrier := retry.NewGRPCRetrier(nil).
        WithMaxAttempts(5).
        WithDelay(200 * time.Millisecond).
        WithBackoff(retry.ExponentialBackoff).
        WithRetryableCodes(codes.Unavailable, codes.ResourceExhausted)

    conn2, _ := grpc.Dial("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(retrier.UnaryClientInterceptor()),
    )
    defer conn2.Close()
}
```

### 基础重试用法

```go
package main

import (
    "context"
    "time"

    "github.com/Tsukikage7/servex/v2/retry"
)

func main() {
    ctx := context.Background()

    // 简单重试
    err := retry.Do(ctx, func() error {
        return callExternalAPI()
    }).Run()

    // 自定义配置
    err = retry.Do(ctx, func() error {
        return callExternalAPI()
    }).
        WithMaxAttempts(5).
        WithDelay(time.Second).
        Run()

    // 带超时
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    err = retry.Do(ctx, func() error {
        return callExternalAPI()
    }).WithMaxAttempts(10).Run()
}
```

## 错误处理

```go
import "github.com/Tsukikage7/servex/v2/retry"

// 检查是否达到最大重试次数
if errors.Is(err, retry.ErrMaxAttempts) {
    // 处理重试耗尽
}
```

## 注意事项

1. **幂等性**：仅对幂等操作使用重试，非幂等操作可能导致重复执行
2. **超时设置**：确保总重试时间不超过请求超时
3. **流式 RPC**：流式 RPC 仅在连接阶段重试，流传输中不重试
4. **服务端重试**：HTTP 服务端重试通常不推荐，请求已到达服务器

## 特性

- **上下文感知**：支持超时和取消
- **灵活配置**：支持自定义退避策略和重试条件
- **链式 API**：流畅的配置方式
- **多协议支持**：Endpoint、HTTP、gRPC
