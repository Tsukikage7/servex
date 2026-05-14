# middleware/trace

`github.com/Tsukikage7/servex/v2/middleware/trace` — 请求链路追踪增强中间件。

统一 trace-id 在日志、响应头、下游调用中的传播，构建于 `observability/logger` 之上。

## 功能

- 从请求中提取或自动生成 trace-id 和 request-id
- 注入到 HTTP 响应头 / gRPC 响应 metadata
- 注入到 logger context（后续日志自动携带 trace_id）
- 提供注入函数，方便客户端调用下游时传播 trace context

## API

### 配置

- `Config` — 链路追踪配置（header 名称、传播列表、logger）
- `DefaultConfig()` — 返回默认配置

### 中间件

- `HTTPMiddleware(cfg)` — HTTP 链路追踪中间件
- `GRPCUnaryInterceptor(cfg)` — gRPC 一元拦截器
- `GRPCStreamInterceptor(cfg)` — gRPC 流式拦截器

### Context 操作

- `TraceIDFromContext(ctx)` — 从 context 获取 trace ID
- `RequestIDFromContext(ctx)` — 从 context 获取 request ID

### 下游传播

- `InjectHTTPHeaders(ctx, req)` — 注入到 HTTP 请求头
- `InjectGRPCMetadata(ctx)` — 注入到 gRPC 出站 metadata

## 使用示例

### HTTP 服务端

```go
import "github.com/Tsukikage7/servex/v2/middleware/trace"

mux := http.NewServeMux()
handler := trace.HTTPMiddleware(nil)(mux) // 使用默认配置
http.ListenAndServe(":8080", handler)
```

### gRPC 服务端

```go
srv := grpc.NewServer(
    grpc.UnaryInterceptor(trace.GRPCUnaryInterceptor(nil)),
    grpc.StreamInterceptor(trace.GRPCStreamInterceptor(nil)),
)
```

### 调用下游 HTTP 服务

```go
func callDownstream(ctx context.Context) {
    req, _ := http.NewRequestWithContext(ctx, "GET", "http://svc-b/api", nil)
    trace.InjectHTTPHeaders(ctx, req)
    http.DefaultClient.Do(req)
}
```

### 调用下游 gRPC 服务

```go
func callGRPC(ctx context.Context, client pb.MyServiceClient) {
    ctx = trace.InjectGRPCMetadata(ctx)
    client.DoSomething(ctx, &pb.Request{})
}
```
