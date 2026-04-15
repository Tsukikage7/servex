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
- 支持 proto option 自动发现公开方法
- 可自定义 protojson 序列化选项和 ServeMux 选项
- 实现 `transport.HealthCheckable` 接口

## 安装

```bash
go get github.com/Tsukikage7/servex/transport/gateway
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
| `WithTrace`             | -                | 启用链路追踪（gRPC + HTTP）    |
| `WithResponse`          | -                | 启用统一响应格式（含细粒度错误码保留） |
| `WithRecovery`          | -                | 启用 panic 恢复（gRPC + HTTP） |
| `WithAuth`              | -                | 启用认证                       |
| `WithPublicMethods`     | -                | 设置公开方法（无需认证）       |
| `WithAutoDiscovery`     | -                | 启用 proto option 自动发现     |
| `WithCORS`              | -                | 启用 CORS（仅 HTTP 端）        |
| `WithRateLimit`         | -                | 启用限流（gRPC + HTTP）        |
| `WithMetrics`           | -                | 启用指标采集（gRPC + HTTP）    |
| `WithLogging`           | -                | 启用请求日志（gRPC + HTTP）    |
| `WithRequestID`         | -                | 启用 Request ID（gRPC + HTTP） |
| `WithClientIP`          | -                | 启用客户端 IP 提取（gRPC + HTTP）|
| `WithTenant`            | -                | 启用多租户解析（gRPC + HTTP）  |
| `WithHTTPTLS`           | -                | 启用 HTTP 端 TLS               |

### 认证与公开方法

由于 gRPC-Gateway 会将 HTTP 请求转换为 gRPC 调用，只需在 gRPC 层添加认证拦截器即可同时保护两种协议：

```go
srv := gateway.New(
    gateway.WithAuth(authenticator),
    gateway.WithPublicMethods(
        "/api.user.v1.AuthService/Login",
        "/api.user.v1.AuthService/*",
    ),
    gateway.WithAutoDiscovery(),
)
```

### 中间件执行顺序

Gateway 对 HTTP 和 gRPC 请求分别应用中间件，执行顺序如下：

1. Recovery（HTTP + gRPC）
2. RequestID（HTTP + gRPC）
3. Logging（HTTP + gRPC）
4. Tracing（HTTP + gRPC）
5. Metrics（HTTP + gRPC）
6. CORS（仅 HTTP）
7. RateLimit（HTTP + gRPC）
8. ClientIP（HTTP + gRPC）
9. Tenant（HTTP + gRPC）
10. Auth（gRPC 拦截器，HTTP 请求通过 gRPC 代理自动受保护）
11. Health（HTTP）

### 完整配置示例

```go
srv := gateway.New(
    gateway.WithLogger(log),
    gateway.WithName("api-gateway"),
    gateway.WithGRPCAddr(":9090"),
    gateway.WithHTTPAddr(":8080"),
    gateway.WithRecovery(),
    gateway.WithRequestID(),
    gateway.WithLogging("/grpc.health.v1.Health/Check"),
    gateway.WithTrace("api-gateway"),
    gateway.WithMetrics(collector),
    gateway.WithCORS(cors.WithAllowOrigins("https://example.com")),
    gateway.WithRateLimit(limiter),
    gateway.WithClientIP(),
    gateway.WithTenant(resolver),
    gateway.WithAuth(authenticator),
    gateway.WithPublicMethods("/api.auth.v1.AuthService/*"),
    gateway.WithResponse(),
)
```

## 统一响应与错误处理

`WithResponse()` 会自动注册 `response.GatewayErrorHandler`，将 gRPC 错误统一转换为 JSON 格式，并支持 i18n。

**细粒度业务 Code 保留：** gRPC-gateway 层的错误码转换是无损的。同一 gRPC code（如 `InvalidArgument`）
可对应多个业务 Code（30001 参数无效 / 30002 缺少参数 / 30003 校验失败）。`response.GRPCStatus` 将
完整 Code 信息以 JSON 嵌入 gRPC status message，`GatewayErrorHandler` 从中精确还原，不会发生降级。

```go
// 推荐方式：使用 WithResponse()，自动处理一切
srv := gateway.New(
    gateway.WithResponse(),
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
