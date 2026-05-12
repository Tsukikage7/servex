# domain/cqrs/middleware

## 导入路径

```go
import "github.com/Tsukikage7/servex/domain/cqrs/middleware"
```

## 简介

`domain/cqrs/middleware` 提供 CQRS 命令和查询处理器的通用中间件，包括日志记录、Prometheus 指标和 OpenTelemetry 链路追踪。所有中间件均为泛型实现，分别适配 `CommandHandler` 和 `QueryHandler`。

## 核心函数

| 函数 | 说明 |
|---|---|
| `CommandLogging[C,R](log, name)` | 命令处理器日志装饰器，记录耗时和错误 |
| `QueryLogging[Q,R](log, name)` | 查询处理器日志装饰器 |
| `CommandMetrics[C,R](name, registerer)` | 命令 Prometheus 指标：总次数、耗时直方图 |
| `QueryMetrics[Q,R](name, registerer)` | 查询 Prometheus 指标 |
| `CommandTracing[C,R](spanName, tracer...)` | 命令 OTel 链路追踪，span 出错时记录 |
| `QueryTracing[Q,R](spanName, tracer...)` | 查询 OTel 链路追踪 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/domain/cqrs"
    "github.com/Tsukikage7/servex/domain/cqrs/middleware"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

type CreateOrderCmd struct{ UserID string }
type CreateOrderResult struct{ OrderID string }

type createOrderHandler struct{}

func (h *createOrderHandler) Handle(ctx context.Context, cmd CreateOrderCmd) (CreateOrderCmd, CreateOrderResult, error) {
    return cmd, CreateOrderResult{OrderID: "order-123"}, nil
}

func main() {
    log := logger.NewNop()

    // 基础 handler
    var handler cqrs.CommandHandler[CreateOrderCmd, CreateOrderResult] = &createOrderHandler{}

    // 应用日志中间件
    handler = middleware.CommandLogging[CreateOrderCmd, CreateOrderResult](
        log, "CreateOrder",
    )(handler)

    // 应用指标中间件
    handler = middleware.CommandMetrics[CreateOrderCmd, CreateOrderResult](
        "create_order", nil,
    )(handler)

    // 应用链路追踪中间件
    handler = middleware.CommandTracing[CreateOrderCmd, CreateOrderResult](
        "cmd.CreateOrder",
    )(handler)

    // 执行命令
    _, result, err := handler.Handle(context.Background(), CreateOrderCmd{UserID: "u-1"})
    if err != nil {
        fmt.Println("错误:", err)
        return
    }
    fmt.Println("订单ID:", result.OrderID)
}
```
