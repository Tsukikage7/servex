# grpcclient

`grpcclient` 包提供功能完整的 gRPC 客户端封装，支持服务发现、TLS/mTLS、重试、熔断、链路追踪、Prometheus 指标、结构化日志、负载均衡以及流式拦截器，并提供 Config 驱动的工厂函数。

## 功能特性

- **服务发现**：通过 `discovery.Discovery` 接口自动解析目标地址（Consul、etcd 等）
- **TLS / mTLS**：集成 `transport/tls`，一行配置启用单向或双向 TLS
- **重试**：内置 Unary 重试拦截器（仅对 `Unavailable` / `DeadlineExceeded` 重试，指数退避）
- **熔断**：接入 `middleware/circuitbreaker`，防止级联故障
- **链路追踪**：自动注入 OpenTelemetry Unary + Stream 拦截器
- **Prometheus 指标**：自动注入 Unary + Stream 指标拦截器
- **结构化日志**：内置 Unary 日志拦截器，记录方法、耗时和错误
- **负载均衡**：支持 `round_robin` / `pick_first` 策略（通过 gRPC 服务配置）
- **流式拦截器**：追踪和指标同时覆盖 Streaming RPC
- **Config 驱动**：`NewFromConfig` / `NewFromConfigWithMetrics` / `NewFromConfigWithDeps` 三档工厂
- **Keepalive & WaitForReady**：开箱即用，默认 60 s / 20 s

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/grpcclient
```

## 快速上手

### 服务发现模式（推荐）

```go
client, err := grpcclient.New(
    grpcclient.WithName("order-client"),          // 日志标识，可选
    grpcclient.WithServiceName("order-service"),  // 必需
    grpcclient.WithDiscovery(consulDiscovery),    // 必需
    grpcclient.WithLogger(log),                   // 必需
    grpcclient.WithRetry(3, 100*time.Millisecond),
    grpcclient.WithLogging(),
    grpcclient.WithTracing("order-service"),
    grpcclient.WithMetrics(metricsCollector),
    grpcclient.WithCircuitBreaker(cb),
    grpcclient.WithBalancer("round_robin"),
)
if err != nil { ... }
defer client.Close()

conn := client.Conn()
orderSvc := pb.NewOrderServiceClient(conn)
resp, err := orderSvc.GetOrder(ctx, &pb.GetOrderRequest{Id: "42"})
```

### Config 驱动模式

```go
// 最简工厂
client, err := grpcclient.NewFromConfig(&grpcclient.Config{
    ServiceName:   "order-service",
    Addr:          "order-service:9090",
    Timeout:       5 * time.Second,
    EnableTracing: true,
    Retry:         &grpcclient.RetryConfig{MaxAttempts: 3, Backoff: 100 * time.Millisecond},
    Balancer:      "round_robin",
})

// 附带 Metrics
client, err := grpcclient.NewFromConfigWithMetrics(cfg, prometheusCollector)

// 附带 Metrics + 熔断器
client, err := grpcclient.NewFromConfigWithDeps(cfg, prometheusCollector, circuitBreaker)
```

### YAML 配置示例

```yaml
grpc_client:
  service_name: order-service
  addr: "order-service:9090"
  timeout: 5s
  enable_tracing: true
  enable_metrics: true
  balancer: round_robin
  retry:
    max_attempts: 3
    backoff: 100ms
  keepalive:
    time: 60s
    timeout: 20s
  tls:
    cert_file: /etc/tls/client.crt
    key_file:  /etc/tls/client.key
    ca_file:   /etc/tls/ca.crt
```

## API

### 构造函数

| 函数 | 说明 |
| --- | --- |
| `New(opts ...Option) (*Client, error)` | 通过服务发现创建客户端（`serviceName`、`discovery`、`logger` 必需，缺少会 panic） |
| `NewFromConfig(cfg, additionalOpts...) (*Client, error)` | Config 驱动，直接连接 `cfg.Addr`，无服务发现 |
| `NewFromConfigWithMetrics(cfg, collector, additionalOpts...)` | 同上，额外注入 Prometheus 收集器 |
| `NewFromConfigWithDeps(cfg, collector, cb, additionalOpts...)` | 同上，额外注入收集器 + 熔断器 |

### Client 方法

```go
func (c *Client) Conn() *grpc.ClientConn  // 获取底层连接，用于创建 stub
func (c *Client) Close() error             // 关闭连接
```

### 配置选项

| 选项 | 默认值 | 说明 |
| --- | --- | --- |
| `WithName(name)` | `gRPC-Client` | 客户端日志标识（可选） |
| `WithServiceName(name)` | — | 目标服务名称（必需） |
| `WithDiscovery(d)` | — | 服务发现实例（必需，`New` 时） |
| `WithLogger(l)` | — | 日志记录器（必需） |
| `WithTLS(cfg)` | — | `*tls.Config`，不设置则使用 insecure |
| `WithRetry(maxAttempts, backoff)` | — | 重试次数 + 退避间隔；仅对 `Unavailable`/`DeadlineExceeded` 重试 |
| `WithCircuitBreaker(cb)` | — | `circuitbreaker.CircuitBreaker` 实例 |
| `WithTracing(serviceName)` | — | 启用 OTel Unary + Stream 拦截器 |
| `WithMetrics(collector)` | — | 启用 Prometheus Unary + Stream 拦截器 |
| `WithLogging()` | — | 启用内置日志拦截器（方法、耗时、错误） |
| `WithBalancer(policy)` | — | `"round_robin"` 或 `"pick_first"` |
| `WithTimeout(d)` | — | Dial 超时 |
| `WithKeepalive(time, timeout)` | 60s / 20s | gRPC keepalive 参数 |
| `WithInterceptors(interceptors...)` | — | 附加自定义 Unary 拦截器 |
| `WithStreamInterceptors(interceptors...)` | — | 附加自定义 Stream 拦截器 |
| `WithDialOptions(opts...)` | — | 附加原生 `grpc.DialOption` |

### Config 结构体

```go
type Config struct {
    ServiceName   string           // 服务名（用于日志/追踪）
    Addr          string           // 直连地址，必需
    TLS           *tlsx.Config     // TLS 配置，nil 则 insecure
    Timeout       time.Duration    // Dial 超时
    Retry         *RetryConfig     // 重试策略
    Balancer      string           // "round_robin" | "pick_first"
    Keepalive     *KeepaliveConfig // Keepalive 参数
    EnableTracing bool             // 启用链路追踪
    EnableMetrics bool             // 启用 Prometheus 指标
}

type RetryConfig struct {
    MaxAttempts int
    Backoff     time.Duration
}

type KeepaliveConfig struct {
    Time    time.Duration
    Timeout time.Duration
}
```

## TLS / mTLS

```go
import tlsx "github.com/Tsukikage7/servex/v2/transport/tls"

// 单向 TLS（验证服务端证书）
tlsCfg, err := tlsx.NewClientTLSConfig(&tlsx.Config{
    CAFile: "/etc/tls/ca.crt",
})

// mTLS（双向验证）
tlsCfg, err := tlsx.NewClientTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/client.crt",
    KeyFile:  "/etc/tls/client.key",
    CAFile:   "/etc/tls/ca.crt",
})

client, err := grpcclient.New(
    grpcclient.WithServiceName("secure-service"),
    grpcclient.WithDiscovery(disc),
    grpcclient.WithLogger(log),
    grpcclient.WithTLS(tlsCfg),
)
```

## 拦截器执行顺序

内置拦截器按以下顺序链接（Unary）：

```
Logging → Retry → CircuitBreaker → Tracing → Metrics → 自定义拦截器
```

Stream 拦截器顺序：

```
Tracing → Metrics → 自定义 Stream 拦截器
```

## 错误处理

| 错误 | 说明 |
| --- | --- |
| `ErrDiscoveryFailed` | 服务发现调用失败 |
| `ErrServiceNotFound` | 指定服务名未找到任何实例 |
| `ErrConnectionFailed` | gRPC 连接建立失败 |

## 许可证

详见项目根目录 LICENSE 文件。
