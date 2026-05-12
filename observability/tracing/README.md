# tracing

提供基于 OpenTelemetry 的分布式链路追踪功能，支持 OTLP HTTP 协议导出、HTTP 中间件和 gRPC 拦截器。

## 安装

```go
import "github.com/Tsukikage7/servex/v2/observability/tracing"
```

## API

### 初始化

#### NewTracer

创建新的链路追踪器。

```go
func NewTracer(cfg *TracingConfig, serviceName, serviceVersion string) (*trace.TracerProvider, error)
```

#### MustNewTracer

创建链路追踪器，失败时 panic。

```go
func MustNewTracer(cfg *TracingConfig, serviceName, serviceVersion string) *trace.TracerProvider
```

### Endpoint 中间件

#### EndpointMiddleware

返回 Endpoint 链路追踪中间件，用于 transport.Endpoint 层追踪。

```go
func EndpointMiddleware(serviceName, operationName string) transport.Middleware
```

#### EndpointTracer

提供可配置的 Endpoint 链路追踪器。

```go
// 创建 Endpoint 链路追踪器
tracer := tracing.NewEndpointTracer("user-service")

// 为不同方法创建中间件
getUserEndpoint = tracer.Middleware("GetUser")(getUserEndpoint)
listUsersEndpoint = tracer.Middleware("ListUsers")(listUsersEndpoint)
```

### HTTP 中间件

#### HTTPMiddleware

返回 HTTP 链路追踪中间件，自动为每个请求创建 span。

```go
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler
```

### gRPC 拦截器

#### UnaryServerInterceptor

返回 gRPC 一元服务端拦截器。

```go
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor
```

#### StreamServerInterceptor

返回 gRPC 流式服务端拦截器。

```go
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor
```

#### UnaryClientInterceptor

返回 gRPC 一元客户端拦截器。

```go
func UnaryClientInterceptor(serviceName string) grpc.UnaryClientInterceptor
```

#### StreamClientInterceptor

返回 gRPC 流式客户端拦截器。

```go
func StreamClientInterceptor(serviceName string) grpc.StreamClientInterceptor
```

### Span 操作

#### StartSpan

在当前 context 中创建新的 span。

```go
func StartSpan(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
```

#### SpanFromContext

从 context 获取当前 span。

```go
func SpanFromContext(ctx context.Context) trace.Span
```

#### AddSpanEvent

向当前 span 添加事件。

```go
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue)
```

#### SetSpanError

设置 span 错误状态。

```go
func SetSpanError(ctx context.Context, err error)
```

#### SetSpanAttributes

设置 span 属性。

```go
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue)
```

### Context 传播

#### InjectHTTPHeaders

将追踪信息注入到 HTTP 请求头，用于跨服务传播。

```go
func InjectHTTPHeaders(ctx context.Context, req *http.Request)
```

#### TraceID / SpanID

从 context 获取 trace ID 或 span ID。

```go
func TraceID(ctx context.Context) string
func SpanID(ctx context.Context) string
```

### gRPC Context 传播

#### InjectGRPCMetadata

将追踪信息注入到 gRPC outgoing metadata。

```go
func InjectGRPCMetadata(ctx context.Context) context.Context
```

#### ExtractGRPCMetadata

从 gRPC incoming metadata 提取追踪信息。

```go
func ExtractGRPCMetadata(ctx context.Context) context.Context
```

### 错误

| 错误                  | 说明               |
| --------------------- | ------------------ |
| `ErrNilConfig`        | 链路追踪配置为空   |
| `ErrEmptyServiceName` | 服务名称为空       |
| `ErrEmptyEndpoint`    | OTLP端点为空       |
| `ErrCreateExporter`   | 创建OTLP导出器失败 |
| `ErrCreateResource`   | 创建资源失败       |

## 使用示例

### HTTP 服务端

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/Tsukikage7/servex/v2/observability/tracing"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    // 初始化 tracer
    cfg := &tracing.TracingConfig{
        Enabled:      true,
        SamplingRate: 1.0,
        OTLP: &tracing.OTLPConfig{
            Endpoint: "localhost:4318",
        },
    }

    tp, err := tracing.NewTracer(cfg, "user-service", "1.0.0")
    if err != nil {
        log.Fatal(err)
    }
    defer tp.Shutdown(context.Background())

    // 创建路由
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", getUsers)
    mux.HandleFunc("/api/users/", getUser)

    // 应用追踪中间件
    handler := tracing.HTTPMiddleware("user-service")(mux)

    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", handler)
}

func getUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 添加自定义属性
    tracing.SetSpanAttributes(ctx, attribute.Int("user.count", 100))

    // 创建子 span
    ctx, span := tracing.StartSpan(ctx, "user-service", "query-database")
    defer span.End()

    // 添加事件
    tracing.AddSpanEvent(ctx, "fetching users from database")

    // 业务逻辑...
    w.Write([]byte(`{"users": []}`))
}

func getUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    userID := r.URL.Path[len("/api/users/"):]

    tracing.SetSpanAttributes(ctx, attribute.String("user.id", userID))

    w.Write([]byte(`{"id": "` + userID + `"}`))
}
```

### gRPC 服务端

```go
package main

import (
    "context"
    "log"
    "net"

    "github.com/Tsukikage7/servex/v2/observability/tracing"
    "google.golang.org/grpc"
    pb "your-project/proto"
)

func main() {
    // 初始化 tracer
    cfg := &tracing.TracingConfig{
        Enabled:      true,
        SamplingRate: 1.0,
        OTLP: &tracing.OTLPConfig{
            Endpoint: "localhost:4318",
        },
    }

    tp, err := tracing.NewTracer(cfg, "order-service", "1.0.0")
    if err != nil {
        log.Fatal(err)
    }
    defer tp.Shutdown(context.Background())

    // 创建 gRPC 服务器，添加追踪拦截器
    server := grpc.NewServer(
        grpc.UnaryInterceptor(tracing.UnaryServerInterceptor("order-service")),
        grpc.StreamInterceptor(tracing.StreamServerInterceptor("order-service")),
    )

    pb.RegisterOrderServiceServer(server, &orderService{})

    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatal(err)
    }

    log.Println("gRPC server starting on :50051")
    server.Serve(lis)
}

type orderService struct {
    pb.UnimplementedOrderServiceServer
}

func (s *orderService) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
    // span 已由拦截器自动创建
    // 可以添加自定义属性
    tracing.SetSpanAttributes(ctx, attribute.String("order.id", req.OrderId))

    // 创建子 span
    ctx, span := tracing.StartSpan(ctx, "order-service", "query-database")
    defer span.End()

    // 业务逻辑...
    return &pb.Order{Id: req.OrderId}, nil
}
```

### gRPC 客户端

```go
package main

import (
    "context"
    "log"

    "github.com/Tsukikage7/servex/v2/observability/tracing"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "your-project/proto"
)

func main() {
    // 初始化 tracer
    cfg := &tracing.TracingConfig{
        Enabled:      true,
        SamplingRate: 1.0,
        OTLP: &tracing.OTLPConfig{
            Endpoint: "localhost:4318",
        },
    }

    tp, err := tracing.NewTracer(cfg, "user-service", "1.0.0")
    if err != nil {
        log.Fatal(err)
    }
    defer tp.Shutdown(context.Background())

    // 创建带追踪拦截器的 gRPC 连接
    conn, err := grpc.Dial("order-service:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(tracing.UnaryClientInterceptor("user-service")),
        grpc.WithStreamInterceptor(tracing.StreamClientInterceptor("user-service")),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewOrderServiceClient(conn)

    // 调用会自动传播追踪上下文
    order, err := client.GetOrder(context.Background(), &pb.GetOrderRequest{
        OrderId: "12345",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Order: %v", order)
}
```

### 跨服务调用 (HTTP)

```go
func callOrderService(ctx context.Context, userID string) error {
    // 创建子 span
    ctx, span := tracing.StartSpan(ctx, "user-service", "call-order-service")
    defer span.End()

    // 创建请求
    req, err := http.NewRequestWithContext(ctx, "GET", "http://order-service/api/orders?user_id="+userID, nil)
    if err != nil {
        tracing.SetSpanError(ctx, err)
        return err
    }

    // 注入追踪头（关键！）
    tracing.InjectHTTPHeaders(ctx, req)

    // 发送请求
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        tracing.SetSpanError(ctx, err)
        return err
    }
    defer resp.Body.Close()

    tracing.SetSpanAttributes(ctx, attribute.Int("http.status_code", resp.StatusCode))

    return nil
}
```

### 记录错误

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, span := tracing.StartSpan(ctx, "order-service", "process-order")
    defer span.End()

    tracing.SetSpanAttributes(ctx, attribute.String("order.id", orderID))

    if err := validateOrder(orderID); err != nil {
        // 记录错误到 span
        tracing.SetSpanError(ctx, err)
        return err
    }

    tracing.AddSpanEvent(ctx, "order validated")

    // 处理订单...
    return nil
}
```

### 获取 Trace ID（用于日志关联）

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    traceID := tracing.TraceID(ctx)
    spanID := tracing.SpanID(ctx)

    // 在日志中包含 trace ID，便于问题追踪
    log.Printf("[trace_id=%s span_id=%s] Processing request", traceID, spanID)

    // 业务逻辑...
}
```

### 配置文件示例

```yaml
tracing:
  enabled: true
  sampling_rate: 0.1 # 10% 采样
  otlp:
    endpoint: jaeger-collector:4318
    headers:
      Authorization: "Bearer token"
```

## 架构图

```
┌─────────────────┐     HTTP Request     ┌─────────────────┐
│  Service A      │────────────────────▶│  Service B      │
│                 │   (with trace ctx)   │                 │
│  ┌───────────┐  │                      │  ┌───────────┐  │
│  │ HTTP      │  │                      │  │ HTTP      │  │
│  │ Middleware│  │                      │  │ Middleware│  │
│  └─────┬─────┘  │                      │  └─────┬─────┘  │
│        │        │                      │        │        │
│        ▼        │                      │        ▼        │
│  ┌───────────┐  │                      │  ┌───────────┐  │
│  │ Business  │  │                      │  │ Business  │  │
│  │ Logic     │  │                      │  │ Logic     │  │
│  └─────┬─────┘  │                      │  └─────┬─────┘  │
└────────┼────────┘                      └────────┼────────┘
         │                                        │
         ▼                                        ▼
    ┌─────────────────────────────────────────────────┐
    │              OTLP Collector (Jaeger/Tempo)       │
    └─────────────────────────────────────────────────┘
```

## 特性

- **OpenTelemetry 标准**: 完全兼容 OpenTelemetry 规范
- **HTTP 中间件**: 自动为 HTTP 请求创建和传播 span
- **gRPC 拦截器**: 支持一元和流式 RPC 的服务端/客户端拦截器
- **跨服务传播**: 支持 W3C Trace Context 传播（HTTP 和 gRPC）
- **采样率控制**: 可配置的采样率
- **便捷 API**: 简化的 span 操作函数
- **错误追踪**: 自动记录错误状态和 gRPC 状态码
