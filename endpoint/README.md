# endpoint

`github.com/Tsukikage7/servex/v2/endpoint`

传输层端点抽象与中间件链组合，为 RPC 方法提供统一的函数签名和中间件编排能力。

## 功能特性

- 统一的端点函数签名，屏蔽传输层差异
- 中间件链式组合，支持任意数量的中间件嵌套
- 提供空操作端点和中间件，方便测试与占位

## API

### 类型定义

```go
// Endpoint 表示单个 RPC 方法
type Endpoint func(ctx context.Context, request any) (response any, err error)

// Middleware 是 Endpoint 中间件
type Middleware func(Endpoint) Endpoint
```

### 函数

| 函数 | 说明 |
|------|------|
| `Chain(outer Middleware, others ...Middleware) Middleware` | 将多个中间件链式组合为单个中间件，按参数顺序从外到内包裹端点 |
| `Nop(ctx context.Context, req any) (any, error)` | 空端点实现，返回空结构体和 nil 错误 |
| `NopMiddleware(next Endpoint) Endpoint` | 空中间件实现，直接透传到下一个端点 |
