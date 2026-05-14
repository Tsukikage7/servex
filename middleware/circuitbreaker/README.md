# middleware/circuitbreaker

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/middleware/circuitbreaker"
```

## 简介

`middleware/circuitbreaker` 实现熔断器（Circuit Breaker）模式。熔断器有三种状态：Closed（正常）、Open（熔断拒绝请求）、HalfOpen（探测恢复）。连续失败超过阈值后开路，等待超时后进入半开状态进行探测，探测成功后关路。同时提供 HTTP 和 gRPC 中间件适配器。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Breaker` | 熔断器实现，实现 `CircuitBreaker` 接口 |
| `New(opts...)` | 创建熔断器 |
| `Execute(ctx, fn)` | 在熔断保护下执行函数 |
| `State()` | 返回当前状态（Closed/Open/HalfOpen） |
| `Reset()` | 手动重置为 Closed |
| `WithFailureThreshold(n)` | 失败阈值（默认 5） |
| `WithSuccessThreshold(n)` | HalfOpen 成功阈值（默认 2） |
| `WithOpenTimeout(d)` | Open 超时时间（默认 10s） |
| `WithIsFailure(fn)` | 自定义失败判断函数 |
| `HTTPMiddleware(breaker)` | HTTP 中间件适配器 |
| `UnaryServerInterceptor(breaker)` | gRPC 一元拦截器 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/Tsukikage7/servex/v2/middleware/circuitbreaker"
)

func main() {
    // 创建熔断器：连续失败 3 次后开路，5s 后尝试恢复
    breaker := circuitbreaker.New(
        circuitbreaker.WithFailureThreshold(3),
        circuitbreaker.WithSuccessThreshold(1),
        circuitbreaker.WithOpenTimeout(5*time.Second),
    )

    ctx := context.Background()

    // 执行受保护的操作
    for i := 0; i < 5; i++ {
        err := breaker.Execute(ctx, func() error {
            // 模拟失败的下游调用
            return fmt.Errorf("service unavailable")
        })
        fmt.Printf("第%d次: 状态=%s, 错误=%v\n", i+1, breaker.State(), err)
    }

    // HTTP 中间件用法
    handler := circuitbreaker.HTTPMiddleware(breaker)(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        }),
    )
    _ = handler

    // 手动重置
    breaker.Reset()
    fmt.Println("重置后状态:", breaker.State()) // closed
}
```
