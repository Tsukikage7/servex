# Recovery

Panic 恢复中间件包，支持 HTTP、gRPC 和 Endpoint 三种类型的 panic 恢复。

## 功能特性

- **HTTP 中间件**: 捕获 HTTP handler 中的 panic，返回 500 状态码
- **gRPC 拦截器**: 捕获 gRPC handler 中的 panic，返回 `codes.Internal` 错误
- **Endpoint 中间件**: 捕获 Endpoint 中的 panic，返回 `PanicError`
- **堆栈捕获**: 自动捕获 panic 堆栈信息用于调试
- **自定义处理**: 支持自定义 panic 处理函数

## 配置选项

| 选项               | 说明                    | 默认值   |
| ------------------ | ----------------------- | -------- |
| `WithLogger(l)`    | 设置日志记录器（必需）  | -        |
| `WithHandler(h)`   | 自定义 panic 处理函数   | 默认处理 |
| `WithStackSize(n)` | 堆栈捕获大小            | 64KB     |
| `WithStackAll(b)`  | 捕获所有 goroutine 堆栈 | false    |

### 自定义处理函数

```go
handler := recovery.HTTPMiddleware(
    recovery.WithLogger(log),
    recovery.WithHandler(func(ctx any, p any, stack []byte) error {
        // ctx 类型：HTTP 为 *http.Request，gRPC/Endpoint 为 context.Context
        // p 为 panic 值
        // stack 为堆栈信息

        // 发送告警通知
        alertService.Send(fmt.Sprintf("Panic: %v\n%s", p, stack))

        // 返回自定义错误
        return errors.New("internal error")
    }),
)(mux)
```

## 日志输出

当 panic 发生时，会记录以下信息：

```json
{
  "level": "error",
  "component": "Recovery",
  "msg": "HTTP 异常已恢复",
  "panic": "something went wrong",
  "method": "GET",
  "path": "/api/users",
  "stack": "goroutine 1 [running]:\n..."
}
```

## PanicError

Endpoint 中间件返回的 `PanicError` 实现了 `error` 和 `Unwrap` 接口：

```go
resp, err := endpoint(ctx, req)
if err != nil {
    var panicErr *recovery.PanicError
    if errors.As(err, &panicErr) {
        // 处理 panic 错误
        fmt.Printf("Panic value: %v\n", panicErr.Value)
        fmt.Printf("Stack: %s\n", panicErr.Stack)

        // 如果 panic 值是 error，可以 Unwrap
        if inner := panicErr.Unwrap(); inner != nil {
            fmt.Printf("Inner error: %v\n", inner)
        }
    }
}
```

## 最佳实践

1. **将 recovery 中间件放在最外层**，确保能捕获所有内层中间件的 panic
2. **始终设置 Logger**，便于排查问题
3. **生产环境考虑自定义 Handler**，发送告警通知
4. **避免在 Handler 中再次 panic**，否则会导致程序崩溃

## API 参考

| 函数                               | 说明                  |
| ---------------------------------- | --------------------- |
| `HTTPMiddleware(opts...)`          | HTTP panic 恢复中间件 |
| `HTTPRecoverFunc(l, h)`            | 简化版 HTTP 恢复函数  |
| `UnaryServerInterceptor(opts...)`  | gRPC 一元拦截器       |
| `StreamServerInterceptor(opts...)` | gRPC 流拦截器         |
| `EndpointMiddleware(opts...)`      | Endpoint 中间件       |
