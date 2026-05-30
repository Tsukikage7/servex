# gateway

`gateway` 包提供 gRPC-Gateway 双协议服务器，同时支持 gRPC 和 HTTP/JSON 两种协议访问同一套服务。

## 功能特性

- 基于 `grpc-ecosystem/grpc-gateway/v2` 实现
- 单次注册同时暴露 gRPC 和 HTTP/JSON 端点
- 内置健康检查（同时支持 HTTP 和 gRPC 协议）
- 支持链路追踪、panic 恢复、认证（gRPC + HTTP 双端）
- 支持 CORS、限流、指标采集、请求日志、Request ID（gRPC + HTTP 双端）
- 支持客户端 IP 提取、多租户解析（gRPC + HTTP 双端）
- 支持 HTTP 端 TLS
- 支持统一响应格式
- 支持 proto option 自动发现公开方法和权限策略
- 可自定义 protojson 序列化选项和 ServeMux 选项
- 实现 `transport.HealthCheckable` 接口

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/gateway
```

## API

### Server

```go
func New(opts ...Option) *Server
func (s *Server) Start(ctx context.Context) error
func (s *Server) Stop(ctx context.Context) error
func (s *Server) Register(services ...Registrar) *Server
func (s *Server) Name() string
func (s *Server) Addr() string            // 返回 gRPC 地址
func (s *Server) HTTPAddr() string         // 返回 HTTP 地址
func (s *Server) GRPCServer() *grpc.Server
func (s *Server) Mux() *runtime.ServeMux
func (s *Server) Health() *health.Health
func (s *Server) HealthEndpoint() *transport.HealthEndpoint
func (s *Server) HealthServer() *health.GRPCServer
```

### Registrar 接口

业务服务需同时实现 gRPC 和 Gateway 注册方法：

```go
type Registrar interface {
    RegisterGRPC(server grpc.ServiceRegistrar)
    RegisterGateway(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error
}
```

### 配置选项

推荐优先使用配置驱动入口，YAML 中描述可序列化的服务行为，Wire 注入 logger、认证器、限流器等运行时对象：

```go
srv, err := gateway.NewFromConfig(&cfg.Gateway, log,
    gateway.WithSecurity(gateway.SecurityConfig{
        Authenticator: authenticator,
        AuthOptions:   []auth.Option{auth.WithAuthorizer(authorizer)},
        RateLimiter:   limiter,
        Tenant:        resolver,
    }),
)
```

`WithObservability` 只表达日志、指标、链路追踪；`WithRuntime` 表达恢复和统一响应。不要把外部可观测性系统做成隐式默认依赖。

```yaml
gateway:
  name: api-gateway
  grpc:
    addr: ":9090"
    enable_reflection: true
  http:
    addr: ":8080"
    read_timeout: 10s
    write_timeout: 10s
    idle_timeout: 120s
  runtime:
    recovery: true
    response: true
  logging:
    enabled: true
    skip_paths:
      - /healthz
      - /metrics
  tracing:
    enabled: false
  metrics:
    enabled: false
    path: /metrics
  cors:
    enabled: false
```

| 选项                    | 默认值           | 说明                           |
| ----------------------- | ---------------- | ------------------------------ |
| `WithLogger`            | -                | 日志记录器（必需）             |
| `WithName`              | `Gateway`        | 服务器名称                     |
| `WithGRPCAddr`          | `:9090`          | gRPC 监听地址                  |
| `WithHTTPAddr`          | `:8080`          | HTTP 监听地址                  |
| `WithConfig`            | -                | 从 GatewayConfig 加载配置      |
| `WithReflection`        | `true`           | 启用 gRPC 反射                 |
| `WithKeepalive`         | `60s, 20s`       | gRPC Keepalive 参数            |
| `WithUnaryInterceptor`  | -                | gRPC 一元拦截器                |
| `WithStreamInterceptor` | -                | gRPC 流拦截器                  |
| `WithGRPCServerOption`  | -                | 自定义 gRPC 服务器选项         |
| `WithHTTPTimeout`       | `30s/30s/120s`   | HTTP 超时（读/写/空闲）        |
| `WithDialOptions`       | -                | gRPC Gateway 拨号选项          |
| `WithServeMuxOptions`   | -                | ServeMux 自定义选项            |
| `WithMarshalOptions`    | -                | protojson 序列化选项           |
| `WithHealthTimeout`     | `5s`             | 健康检查超时                   |
| `WithHealthOptions`     | -                | 健康检查扩展选项               |
| `WithObservability`     | -                | 观测能力：追踪、指标、日志     |
| `WithRuntime`           | -                | 运行时行为：panic 恢复、统一响应 |
| `WithSecurity`          | -                | 安全能力：认证、CORS、限流、客户端 IP、多租户 |
| `WithHTTPTLS`           | -                | 启用 HTTP 端 TLS               |
| `WithGRPCTLS`           | -                | 启用 Gateway 回连 gRPC TLS     |
| `WithHTTPMiddleware`    | -                | 自定义 HTTP 中间件             |

### 认证与声明式策略

由于 gRPC-Gateway 会将 HTTP 请求转换为 gRPC 调用，只需在 gRPC 层添加认证拦截器即可同时保护两种协议：

```go
srv := gateway.New(
    gateway.WithSecurity(gateway.SecurityConfig{
        Authenticator: authenticator,
        AuthOptions:   []auth.Option{auth.WithAuthorizer(authorizer)},
    }),
)
```

配置 `WithSecurity(gateway.SecurityConfig{Authenticator: ...})` 后，Gateway 会自动读取 `auth/proto` 中声明的 `public`、`permissions` 和 `all_permissions`：

- `public: true`：跳过认证，适合登录、注册、健康检查等公共接口。
- `permissions`：认证通过后交给 `auth.Authorizer` 校验。默认是 OR 语义，满足任一权限即可。
- `all_permissions: true`：将 `permissions` 切换为 AND 语义，必须全部满足。
- 没有 proto option：不公开，也不附加权限要求，按普通认证接口处理。

权限字符串默认按 `resource:action` 解析，例如 `orders:create` 会传给授权器
`auth.Target{Resource: "orders", Action: "create"}`。如果方法声明了 `permissions`
但 `AuthOptions` 没有显式传入 `auth.WithAuthorizer(...)`，框架会拒绝访问，避免策略声明失效。

### 中间件执行顺序

Gateway 对 HTTP 和 gRPC 请求分别应用中间件，执行顺序如下：

1. Recovery（HTTP + gRPC）
2. Logging（HTTP + gRPC）
3. Tracing（HTTP + gRPC）
4. Metrics（HTTP + gRPC）
5. CORS（仅 HTTP）
6. RateLimit（HTTP + gRPC）
7. ClientIP（HTTP + gRPC）
8. Tenant（HTTP + gRPC）
9. Auth（gRPC 拦截器，HTTP 请求通过 gRPC 代理自动受保护）
10. Health（HTTP）

### 完整配置示例

```go
srv := gateway.New(
    gateway.WithLogger(log),
    gateway.WithName("api-gateway"),
    gateway.WithGRPCAddr(":9090"),
    gateway.WithHTTPAddr(":8080"),
    gateway.WithObservability(gateway.ObservabilityConfig{
        TracingService:   "api-gateway",
        TracingSkipPaths: []string{"/metrics", "/healthz"},
        Metrics:          collector,
        Logging:          true,
        LoggingSkipPaths: []string{"/grpc.health.v1.Health/Check"},
    }),
    gateway.WithRuntime(gateway.RuntimeConfig{
        Recovery: true,
        Response: true,
    }),
    gateway.WithSecurity(gateway.SecurityConfig{
        Authenticator: authenticator,
        AuthOptions:   []auth.Option{auth.WithAuthorizer(authorizer)},
        CORS:          true,
        CORSOptions:   []cors.Option{cors.WithAllowOrigins("https://example.com")},
        RateLimiter:   limiter,
        ClientIP:      true,
        Tenant:        resolver,
    }),
)
```

## 统一响应与错误处理

`WithRuntime(gateway.RuntimeConfig{Response: true})` 会自动注册 `response.GatewayErrorHandler`，将 gRPC 错误统一转换为 JSON 格式，并支持 i18n。

**细粒度业务 Code 保留：** gRPC-gateway 层的错误码转换是无损的。同一 gRPC code（如 `InvalidArgument`）
可对应多个业务 Code（30001 参数无效 / 30002 缺少参数 / 30003 校验失败）。`response.GRPCStatus` 将
完整 Code 信息以 JSON 嵌入 gRPC status message，`GatewayErrorHandler` 从中精确还原，不会发生降级。

```go
// 推荐方式：通过 WithRuntime 启用统一响应
srv := gateway.New(
    gateway.WithRuntime(gateway.RuntimeConfig{Response: true}),
    // ...
)

// 也可以手动注册（行为等价）
srv := gateway.New(
    gateway.WithServeMuxOptions(response.GatewayServeMuxOption()),
)
```

## 启动流程

1. 启动 gRPC 服务器，注册所有业务服务和健康检查
2. 建立 Gateway 内部连接（gRPC 客户端连接到本地 gRPC 服务器）
3. 注册 Gateway 处理器，启动 HTTP 服务器

## 许可证

详见项目根目录 LICENSE 文件。
