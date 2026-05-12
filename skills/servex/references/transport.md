# servex 传输层

## httpserver — 带认证的 HTTP 服务器

```go
// 适用场景：需要 JWT 认证、结构化日志、链路追踪的 HTTP 服务
srv := httpserver.New(mux,
    httpserver.WithLogger(log),
    httpserver.WithAddr(":8080"),
    httpserver.WithRecovery(),
    httpserver.WithLogging("/healthz"),           // 跳过健康检查路径的日志
    httpserver.WithTrace("my-service"),           // OpenTelemetry 服务名
    httpserver.WithAuth(authenticator, "/api/login"), // 公开路径白名单
)
if err := srv.Start(ctx); err != nil {
    log.Error(ctx, "启动失败", err)
}
```

完整示例：`docs/superpowers/examples/httpserver/main.go`

### Config 驱动创建（推荐用于生产）

```yaml
# config.yaml
httpserver:
  name: api
  addr: ":8080"
  recovery: true
  logging: true
  log_skip_paths: ["/healthz"]
  tracing: "my-service"
  client_ip: true
  tls:
    cert_file: /etc/tls/server.crt
    key_file:  /etc/tls/server.key
```

```go
var cfg httpserver.Config
// 通过 config 包加载后
srv := httpserver.NewFromConfig(mux, &cfg, log,
    // Config 无法表达的运行时选项（Auth、Tenant 等）可通过 additionalOpts 补充
    httpserver.WithAuth(authenticator, "/api/login"),
)
if err := srv.Start(ctx); err != nil {
    log.Error(ctx, "启动失败", err)
}
```

**关键选项：**
- `WithAddr(addr)` — 监听地址，默认 `:8080`
- `WithLogger(log)` — logger（必须）
- `WithRecovery()` — 捕获 panic，返回 500
- `WithLogging(skipPaths...)` — 结构化访问日志，跳过指定路径
- `WithTrace(serviceName)` — OpenTelemetry 中间件
- `WithAuth(authenticator, publicPaths...)` — 认证中间件，白名单外均需认证
- `WithMiddlewares(mws...)` — 注入自定义 `func(http.Handler) http.Handler`
- `WithClientIP()` — 提取客户端真实 IP，写入 context
- `WithVersion(v)` — 设置服务版本，注入到健康检查响应的 `version` 字段（v2.0.5+）
- `NewFromConfig(handler, cfg, log, additionalOpts...)` — Config 驱动工厂，Config 字段自动转换为选项

### Router + Handle — 类型安全路由

```go
router := httpserver.NewRouter()

// 公开路由
router.POST("/login", httpserver.Handle(loginHandler))

// 带认证的 API 分组（继承 router 的所有中间件）
api := router.Group("/api/v1", jwtMiddleware)
api.GET("/users/{id}", httpserver.HandleWith(decodeID, getUser))
api.POST("/users", httpserver.Handle(createUser))

// 嵌套分组 + 额外中间件
admin := api.Group("/admin", adminOnlyMiddleware)
admin.DELETE("/users/{id}", httpserver.HandleWith(decodeID, deleteUser))

srv := httpserver.New(router,
    httpserver.WithLogger(log),
    httpserver.WithRecovery(),
)
```

- `NewRouter(mws...)` — 创建根路由器，可选传入全局中间件
- `router.Group(prefix, mws...)` — 创建子路由分组，继承父级中间件链
- `router.Use(mws...)` — 追加中间件（影响后续注册的路由）
- `router.GET/POST/PUT/PATCH/DELETE(path, handler, routeMws...)` — 注册路由

**Handle / HandleWith — 类型安全端点适配器：**

```go
// Handle：请求参数从请求体自动解码（适用于 POST/PUT/PATCH）
router.POST("/users", httpserver.Handle(
    func(ctx context.Context, req CreateUserReq) (*UserResp, error) {
        return svc.CreateUser(ctx, req)
        // 返回：{"code":0,"message":"成功","data":{"id":1,"name":"Alice"}}
    },
))

// HandleWith：自定义解码器（适用于 GET/DELETE，需从路径参数、查询字符串提取数据）
router.GET("/users/{id}", httpserver.HandleWith(
    func(ctx context.Context, r *http.Request) (GetUserReq, error) {
        return GetUserReq{ID: r.PathValue("id")}, nil
    },
    func(ctx context.Context, req GetUserReq) (*UserResp, error) {
        return svc.GetUser(ctx, req.ID)
    },
))
```

### Registrar — 服务注册模式

```go
// Registrar 接口
type Registrar interface {
    RegisterHTTP(router *Router)
}

// 实现 Registrar 接口
type UserRoutes struct { svc *UserService }

func (r *UserRoutes) RegisterHTTP(router *httpserver.Router) {
    api := router.Group("/api/v1/users")
    api.POST("/", httpserver.Handle(r.svc.Create))
    api.GET("/{id}", httpserver.HandleWith(decodeID, r.svc.Get))
}

// 通过 Register 方法链式注册多个服务
router := httpserver.NewRouter()
srv := httpserver.New(router,
    httpserver.WithLogger(log),
    httpserver.WithRecovery(),
)
srv.Register(&UserRoutes{svc: userSvc}, &OrderRoutes{svc: orderSvc})
```

- `srv.Register(registrars...)` — 链式注册多个 `Registrar` 实现，路由自动注册到 Router 上

## httpclient — 带负载均衡的 HTTP 客户端

```go
// 注意：WithServiceName、WithDiscovery、WithLogger 缺少任一将 panic（非 error 返回）
client, err := httpclient.New(
    httpclient.WithName("order-client"),        // 客户端标识，用于日志（可选）
    httpclient.WithLogger(log),
    httpclient.WithTimeout(5 * time.Second),
    httpclient.WithServiceName("order-service"), // 服务发现的 key（必须）
    httpclient.WithDiscovery(disc),              // discovery.Discovery 实现（consul/etcd/静态）
    httpclient.WithBalancer(&httpclient.RoundRobinBalancer{}),
)

// Do(ctx, method, path, body) — host 由 balancer/discovery 决定
resp, err := client.Do(ctx, http.MethodGet, "/api/orders", nil)
```

完整示例：`docs/superpowers/examples/httpclient/main.go`

**注意事项：**
- `httpclient.New` 构建时立即调用 `Discover`，地址列表不能为空
- 无真实注册中心时，可实现 `discovery.Discovery` 接口传入静态地址列表
- `RoundRobinBalancer` 是默认值，也可使用 `RandomBalancer`

## grpcserver — gRPC 服务器（片段）

```go
srv := grpcserver.New(
    grpcserver.WithLogger(log),
    grpcserver.WithAddr(":9090"),
    grpcserver.WithRecovery(),
    grpcserver.WithTrace("my-service"),
)
// 注册 gRPC 服务
pb.RegisterMyServiceServer(srv.Server(), &myServiceImpl{})
srv.Start(ctx)
```

### Metrics 与限流

```go
collector, _ := metrics.New(metricsCfg)
limiter := ratelimit.NewTokenBucket(100, 200) // 令牌桶：100 QPS，峰值 200

srv := grpcserver.New(
    grpcserver.WithLogger(log),
    grpcserver.WithAddr(":9090"),
    grpcserver.WithRecovery(),
    grpcserver.WithTrace("my-service"),
    grpcserver.WithMetrics(collector),    // Prometheus 指标收集（方法名、状态码、耗时）
    grpcserver.WithRateLimit(limiter),    // 限流，超限返回 ResourceExhausted
)
```

- `WithMetrics(collector *metrics.PrometheusCollector)` — 启用 Prometheus 指标收集，gRPC 端记录方法名、状态码和耗时
- `WithRateLimit(limiter ratelimit.Limiter)` — 启用限流，超限时返回 `codes.ResourceExhausted` 错误
- `WithVersion(v)` — 设置服务版本，注入到健康检查响应的 `version` 字段（v2.0.5+）

## ginserver / echoserver / hertzserver — 框架适配（片段）

```go
// Gin
engine := gin.New()
ginserver.New(engine).ApplyMiddlewares(
    ginserver.Recovery(),
    ginserver.RequestID(),
)

// Echo
e := echo.New()
echoserver.New(e).ApplyMiddlewares(
    echoserver.Recovery(),
)
```

## websocket — WebSocket 服务端（片段）

```go
handler := websocket.NewHandler(
    websocket.WithOnConnect(func(conn *websocket.Conn) {
        log.Info("新连接:", conn.ID())
    }),
    websocket.WithOnMessage(func(conn *websocket.Conn, msg []byte) {
        conn.Send(msg) // echo
    }),
    websocket.WithOnDisconnect(func(conn *websocket.Conn, err error) {}),
)

mux.Handle("/ws", handler)
```

## sse — Server-Sent Events（片段）

```go
handler := sse.NewHandler(
    sse.WithOnConnect(func(client *sse.Client) {
        client.Send(&sse.Event{Data: "connected"})
    }),
)

mux.Handle("/events", handler)
// 向所有客户端广播
handler.Broadcast(&sse.Event{Event: "update", Data: payload})
```

## gateway — gRPC + HTTP 双协议服务器（gRPC-Gateway）

```go
// 创建 Gateway 服务器（同时暴露 gRPC 和 HTTP 端口）
srv := gateway.New(
    gateway.WithName("order-service"),
    gateway.WithGRPCAddr(":9090"),
    gateway.WithHTTPAddr(":8080"),
    gateway.WithLogger(log),
    gateway.WithRecovery(),                    // panic 恢复（双端）
    gateway.WithTrace("order-service"),         // 链路追踪（双端）
    gateway.WithResponse(),                     // 统一响应格式
    gateway.WithReflection(true),               // gRPC 反射
    gateway.WithAuth(authenticator, auth.WithAuthorizer(authorizer)), // 认证与 proto 权限策略（双端）
    gateway.WithReadinessChecker(dbChecker),    // 就绪检查
)

// 注册服务（需实现 gateway.Registrar 接口）
srv.Register(&OrderService{}, &UserService{})

// 启动
if err := srv.Start(ctx); err != nil { ... }
defer srv.Stop(ctx)
```

### 中间件选项

**CORS（仅 HTTP 端）**

```go
gateway.WithCORS(
    cors.WithAllowOrigins("https://example.com", "https://app.example.com"),
    cors.WithAllowCredentials(true),
)
```

**限流（双端）**

```go
// 令牌桶：100 QPS，峰值 200
limiter := ratelimit.NewTokenBucket(100, 200)
gateway.WithRateLimit(limiter)
```

**Metrics（双端）**

```go
collector, _ := metrics.New(metricsCfg)
gateway.WithMetrics(collector)
// HTTP 端：记录方法、路径、状态码、耗时
// gRPC 端：记录方法名、状态码、耗时
```

**请求日志（双端）**

```go
// 跳过健康检查路径/方法
gateway.WithLogging("/grpc.health.v1.Health/Check")
```

**多租户解析（双端）**

```go
gateway.WithTenant(resolver,
    tenant.WithTokenExtractor(tenant.HeaderTokenExtractor("X-Tenant-ID")),
)
```

**客户端 IP 提取（双端）**

```go
// HTTP 端：X-Forwarded-For / X-Real-IP / RemoteAddr
// gRPC 端：metadata + peer 地址
gateway.WithClientIP(clientip.WithTrustPrivateProxies())
```

**Request ID（双端）**

```go
// 自动生成或透传请求 ID，注入 context 并写入响应头/metadata
gateway.WithRequestID()
```

**HTTP TLS**

```go
tlsCfg, _ := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile: "server.crt",
    KeyFile:  "server.key",
})
gateway.WithHTTPTLS(tlsCfg)
// gRPC 端 TLS 通过 WithGRPCServerOption 单独配置
```

**关键类型：**
- `gateway.New(opts...) *Server` — 构造器
- `gateway.Registrar` — 服务注册接口（`RegisterGRPC` + `RegisterGateway`）
- `server.Register(services...)` — 注册业务服务
- 内置健康检查：`/healthz`（存活）、`/readyz`（就绪）
- `WithConfig(transport.GatewayConfig)` — 从配置结构体设置
- `WithVersion(v)` — 设置服务版本，注入到健康检查响应（v2.0.5+）

**Gateway 统一错误响应（v2.0.4+）：**

gateway 的 gRPC 错误会通过 `response.GatewayErrorHandler` 转换为统一 JSON 格式（支持 i18n）。
此处理器已从 gateway 包迁移到 `transport/response` 包：

```go
import "github.com/Tsukikage7/servex/v2/transport/response"

srv := gateway.New(
    gateway.WithServeMuxOptions(response.GatewayServeMuxOption()),
    // ... 其他选项
)
```

- `response.GatewayErrorHandler` — 将 gRPC Status 转为统一 JSON 错误响应，读取 Accept-Language 支持 i18n
- `response.GatewayServeMuxOption()` — 返回注册了统一错误处理器的 `runtime.ServeMuxOption`

**细粒度业务 Code 保留（v2.0.7+）：**

同一 gRPC code（如 `InvalidArgument`）可对应多个业务 Code（30001/30002/30003），纯粗粒度反向映射会丢失细节。
`GRPCStatus` 将完整 Code 信息以 JSON 格式嵌入 gRPC status message，`FromGRPCStatus`/`GatewayErrorHandler` 优先从中恢复：

```json
{"code":30002,"key":"error.missing_param","message":"缺少必需参数","http":400}
```

非 servex 来源的 gRPC 错误按原生 gRPC code 粗粒度映射。

**拦截器执行顺序（gRPC 端）：** Recovery → Tracing → RequestID → Logging → Metrics → RateLimit → ClientIP → Tenant → Auth

## grpcclient — gRPC 客户端（服务发现/重试/熔断/追踪/负载均衡）

```go
// 服务发现模式（serviceName、discovery、logger 缺少任一将 panic）
client, err := grpcclient.New(
    grpcclient.WithName("order-client"),
    grpcclient.WithServiceName("order-service"),  // 必需
    grpcclient.WithDiscovery(disc),               // 必需
    grpcclient.WithLogger(log),                   // 必需
    grpcclient.WithRetry(3, 100*time.Millisecond),  // 重试：仅 Unavailable/DeadlineExceeded
    grpcclient.WithCircuitBreaker(cb),              // 熔断
    grpcclient.WithLogging(),                       // 内置日志拦截器
    grpcclient.WithTracing("order-service"),        // OTel Unary + Stream
    grpcclient.WithMetrics(prometheusCollector),    // Prometheus Unary + Stream
    grpcclient.WithBalancer("round_robin"),         // round_robin | pick_first
)
if err != nil { ... }
defer client.Close()

// 获取底层 gRPC 连接，创建 stub
conn := client.Conn()
orderSvc := pb.NewOrderServiceClient(conn)
resp, err := orderSvc.GetOrder(ctx, &pb.GetOrderRequest{Id: "42"})
```

```go
// Config 驱动（直连，不走服务发现）
client, err := grpcclient.NewFromConfig(&grpcclient.Config{
    ServiceName:   "order-service",
    Addr:          "order-service:9090",
    Timeout:       5 * time.Second,
    EnableTracing: true,
    EnableMetrics: true,
    Balancer:      "round_robin",
    Retry:         &grpcclient.RetryConfig{MaxAttempts: 3, Backoff: 100 * time.Millisecond},
    Keepalive:     &grpcclient.KeepaliveConfig{Time: 60 * time.Second, Timeout: 20 * time.Second},
    TLS: &tlsx.Config{          // 可选；nil 则 insecure
        CAFile: "/etc/tls/ca.crt",
    },
})

// 附带 Metrics collector
client, err := grpcclient.NewFromConfigWithMetrics(cfg, prometheusCollector)

// 附带 Metrics + 熔断器
client, err := grpcclient.NewFromConfigWithDeps(cfg, prometheusCollector, circuitBreaker)
```

```go
// TLS / mTLS
import tlsx "github.com/Tsukikage7/servex/v2/transport/tls"

tlsCfg, err := tlsx.NewClientTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/client.crt",  // mTLS 需要
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

**关键类型：**
- `grpcclient.New(opts...) (*Client, error)` — 服务发现模式，`serviceName`/`discovery`/`logger` 必需
- `grpcclient.NewFromConfig(cfg, opts...)` — Config 驱动直连
- `grpcclient.NewFromConfigWithMetrics(cfg, collector, opts...)` — Config + Metrics
- `grpcclient.NewFromConfigWithDeps(cfg, collector, cb, opts...)` — Config + Metrics + 熔断
- `client.Conn() *grpc.ClientConn` — 获取底层连接，用于创建 stub
- `WithTLS(cfg)` — 启用 TLS/mTLS
- `WithRetry(maxAttempts, backoff)` — 重试（仅 Unavailable/DeadlineExceeded）
- `WithCircuitBreaker(cb)` — 熔断器
- `WithTracing(serviceName)` — OTel Unary + Stream 拦截器
- `WithMetrics(collector)` — Prometheus Unary + Stream 拦截器
- `WithLogging()` — 内置日志拦截器
- `WithBalancer(policy)` — `"round_robin"` | `"pick_first"`
- `WithInterceptors(...)` — 自定义 Unary 拦截器
- `WithStreamInterceptors(...)` — 自定义 Stream 拦截器
- `WithDialOptions(...)` — 额外原生 dial 选项

**拦截器顺序（Unary）：** Logging → Retry → CircuitBreaker → Tracing → Metrics → 自定义

## health — 健康检查

```go
// 创建健康检查管理器
h := health.New(
    health.WithTimeout(5 * time.Second),
    health.WithLivenessChecker(health.NewAlwaysUpChecker("self")),
    health.WithReadinessChecker(
        health.NewDBChecker("postgres", dbPinger),
        health.NewRedisChecker("redis", redisPinger),
    ),
)

// 动态添加检查器
h.AddReadinessChecker(health.NewPingChecker("es", esPinger))

// 自定义检查器
h.AddReadinessChecker(health.NewCheckerFunc("custom", func(ctx context.Context) health.CheckResult {
    if err := doCheck(); err != nil {
        return health.CheckResult{Status: health.StatusDown, Message: err.Error()}
    }
    return health.CheckResult{Status: health.StatusUp}
}))

// 注册 HTTP 路由（/healthz + /readyz）
handler := health.NewHTTPHandler(h)
handler.RegisterRoutes(mux)

// 或使用中间件（自动拦截 /healthz、/readyz）
srv := httpserver.New(health.Middleware(h)(mux))

// 判断是否健康
if h.IsHealthy(ctx) { ... }
```

**关键类型：**
- `health.Health` — 健康检查管理器（`Liveness`, `Readiness`, `IsHealthy`）
- `health.Checker` — 检查器接口（`Name() string`, `Check(ctx) CheckResult`）
- `health.Response` — 响应结构体，包含 `Status`、`Checks`、`Version`（v2.0.5+）字段
- 内置检查器：`NewDBChecker`、`NewRedisChecker`、`NewPingChecker`、`NewAlwaysUpChecker`、`NewCompositeChecker`
- `health.Middleware(h)` — HTTP 中间件，自动拦截 `/healthz`、`/readyz`
- `WithVersion(v)` — 设置服务版本，健康检查响应中会携带 `"version"` 字段（v2.0.5+）
- 状态：`StatusUp`、`StatusDown`、`StatusUnknown`

## response — 统一响应格式

```go
// 统一响应体：{"code": 0, "message": "成功", "data": {...}}
resp := response.OK(user)                           // 成功
resp := response.OKWithMessage(user, "创建成功")
resp := response.Fail[any](response.CodeNotFound)    // 失败
resp := response.FailWithMessage[any](response.CodeInvalidParam, "ID 不能为空")
resp := response.FailWithError[any](err)             // 从 error 提取错误码

// 分页响应：{"code": 0, "message": "成功", "data": [...], "pagination": {...}}
resp := response.Paged(paginationResult)

// 业务错误
err := response.CodeNotFound.ToError()
err := response.CodeInvalidParam.ToError().WithMessage("用户名已存在")
err := response.CodeDatabaseError.ToError().WithCause(dbErr)

// 提取错误信息
code := response.ExtractCode(err)      // 提取错误码
msg := response.ExtractMessage(err)     // 提取消息（5xxxx 隐藏详情）
msg := response.ExtractMessageUnsafe(err) // 完整消息（仅用于日志）
```

**错误码规范：**
- `0` — 成功
- `1xxxx` — 通用错误（`CodeUnknown`, `CodeCanceled`, `CodeTimeout`）
- `2xxxx` — 认证/授权（`CodeUnauthorized`, `CodeForbidden`, `CodeTokenExpired`）
- `3xxxx` — 参数（`CodeInvalidParam`, `CodeMissingParam`, `CodeValidationFailed`）
- `4xxxx` — 资源（`CodeNotFound`, `CodeAlreadyExists`, `CodeConflict`）
- `5xxxx` — 内部（`CodeInternal`, `CodeDatabaseError`）
- `6xxxx` — 外部服务（`CodeServiceUnavailable`, `CodeUpstreamError`）

**关键类型：**
- `response.Response[T]` / `response.PagedResponse[T]` — 统一响应体（实现 `Envelope` 接口）
- `response.Code` — 错误码（含 `Num`、`Message`、`Key`、`Kind`）
- `response.Code.ToError()` — 转换为统一错误类型
- `response.NewCodeWithKind(num, key, message, kind)` — 推荐的自定义错误码入口，HTTP/gRPC 由 Kind 推导

## transport/graphql — GraphQL 服务器适配

```go
// 1. 定义 GraphQL Schema（使用 graphql-go/graphql）
userType := graphql.NewObject(graphql.ObjectConfig{
    Name: "User",
    Fields: graphql.Fields{
        "id":   &graphql.Field{Type: graphql.String},
        "name": &graphql.Field{Type: graphql.String},
    },
})

queryType := graphql.NewObject(graphql.ObjectConfig{
    Name: "Query",
    Fields: graphql.Fields{
        "user": &graphql.Field{
            Type: userType,
            Args: graphql.FieldConfigArgument{
                "id": &graphql.ArgumentConfig{Type: graphql.String},
            },
            // 使用 WrapResolve 为单个字段添加中间件
            Resolve: gqlserver.WrapResolve(
                func(p graphql.ResolveParams) (any, error) {
                    id, _ := p.Args["id"].(string)
                    return findUser(p.Context, id)
                },
                gqlserver.LoggingMiddleware(log),
                gqlserver.TracingMiddleware("user-service"),
            ),
        },
    },
})

schema, _ := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})

// 2. 创建 GraphQL 服务器
srv := gqlserver.New(schema,
    gqlserver.WithLogger(log),
    gqlserver.WithConfig(&gqlserver.Config{
        Pretty:     false,
        Playground: true,     // 启用 GraphiQL UI
        Path:       "/graphql",
    }),
    // 全局中间件（对所有 resolve 函数生效）
    gqlserver.WithMiddleware(
        gqlserver.RecoveryMiddleware(log),
        gqlserver.LoggingMiddleware(log),
        gqlserver.TracingMiddleware("my-service"),
    ),
)

// 3. 注册路由
mux.Handle("/graphql", srv.Handler())
if cfg.Playground {
    mux.Handle("/playground", srv.PlaygroundHandler())
}
```

**内置中间件（resolve 层）：**

```go
// 日志：记录字段名和耗时
gqlserver.LoggingMiddleware(log)

// 链路追踪：为每次 resolve 创建 OTel span
gqlserver.TracingMiddleware("service-name")

// Panic 恢复：防止单个 resolve 崩溃整个服务
gqlserver.RecoveryMiddleware(log)

// 链接多个中间件
combined := gqlserver.ChainMiddleware(
    gqlserver.RecoveryMiddleware(log),
    gqlserver.LoggingMiddleware(log),
    gqlserver.TracingMiddleware("svc"),
)
```

**关键类型：**
- `graphql.New(schema, opts...) *Server` — 创建服务器
- `server.Handler() http.Handler` — GraphQL 请求处理器（支持 GET/POST）
- `server.PlaygroundHandler() http.Handler` — GraphiQL 交互式 UI
- `graphql.Config` — 配置（`Pretty bool`, `Playground bool`, `Path string`）
- `graphql.Middleware` — `func(ResolveFunc) ResolveFunc`，resolve 层中间件
- `graphql.WrapResolve(fn, mw...)` — 将中间件应用到单个 resolve 函数
- `graphql.ChainMiddleware(outer, others...)` — 链接多个中间件
- 内置中间件：`LoggingMiddleware(log)`, `TracingMiddleware(serviceName)`, `RecoveryMiddleware(log)`
- `graphql.ErrorHandlerFunc` — `func(ctx, []gqlerrors.FormattedError) []gqlerrors.FormattedError`，自定义错误处理

**注意：**
- Schema 定义使用 `github.com/graphql-go/graphql`，servex 提供服务器适配层
- `WithMiddleware` 为全局中间件（所有 resolve），`WrapResolve` 为字段级中间件
- `DefaultConfig()` 默认启用 Playground，路径为 `/graphql`

## transport 工具

### BuildMethodSkipper — 方法跳过器

```go
// 构建方法跳过器，支持精确匹配和前缀通配
skipper := transport.BuildMethodSkipper([]string{
    "/health",                           // 精确匹配
    "/api/public/*",                     // 前缀通配
    "/grpc.health.v1.Health/Check",      // gRPC 健康检查
    "/api.auth.v1.AuthService/*",        // gRPC 服务级通配
})

ok := skipper("/api/public/docs")       // true
ok = skipper("/api/private/users")      // false
```

- `transport.BuildMethodSkipper(methods) MethodSkipper` — 返回 `func(method string) bool`，用于自定义中间件跳过指定路径/方法
- Gateway 的公开接口由 proto option 声明，不再通过手动方法白名单配置

## transport/tls — TLS 配置工具（tlsx）

```go
import tlsx "github.com/Tsukikage7/servex/v2/transport/tls"
```

### httpserver 启用 TLS

```go
tlsCfg, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/server.crt",
    KeyFile:  "/etc/tls/server.key",
})
if err != nil {
    log.Fatal(err)
}

srv := httpserver.New(mux,
    httpserver.WithAddr(":443"),
    httpserver.WithTLS(tlsCfg),
)
```

### grpcserver 启用 TLS

```go
tlsCfg, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/server.crt",
    KeyFile:  "/etc/tls/server.key",
})
if err != nil {
    log.Fatal(err)
}

grpcSrv := grpcserver.New(
    grpcserver.WithAddr(":9443"),
    grpcserver.WithTLS(tlsCfg),
)
```

### mTLS（双向 TLS）

```go
// 服务端：强制验证客户端证书
serverTLS, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile:   "/etc/tls/server.crt",
    KeyFile:    "/etc/tls/server.key",
    CAFile:     "/etc/tls/ca.crt",       // 客户端证书的签发 CA
    ClientAuth: "require_and_verify",     // 强制双向验证
})

// 客户端：提供客户端证书
clientTLS, err := tlsx.NewClientTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/client.crt",
    KeyFile:  "/etc/tls/client.key",
    CAFile:   "/etc/tls/ca.crt",         // 验证服务端证书
})

httpClient := &http.Client{
    Transport: &http.Transport{TLSClientConfig: clientTLS},
}
```

### 指定最低 TLS 版本

```go
tlsCfg, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile:   "server.crt",
    KeyFile:    "server.key",
    MinVersion: "1.3",  // 仅允许 TLS 1.3
})
```

### 从配置文件加载（与 config 包集成）

```yaml
# config.yaml
tls:
  cert_file: /etc/tls/server.crt
  key_file:  /etc/tls/server.key
  ca_file:   /etc/tls/ca.crt
  min_version: "1.2"
  client_auth: require_and_verify
```

```go
var cfg tlsx.Config
// 通过 config 包加载后
tlsCfg, err := tlsx.NewServerTLSConfig(&cfg)
```

**关键 API：**
- `tlsx.NewServerTLSConfig(cfg)` — 服务端 TLS 配置（需要 CertFile + KeyFile）
- `tlsx.NewClientTLSConfig(cfg)` — 客户端 TLS 配置（CertFile/KeyFile 可选，用于 mTLS）
- `tlsx.NewTLSConfig(cfg)` — 通用，等同 NewServerTLSConfig
- `httpserver.WithTLS(tlsCfg)` — httpserver 启用 TLS
- `grpcserver.WithTLS(tlsCfg)` — grpcserver 启用 TLS

**ClientAuth 选项：** `""` (不验证) | `"request"` | `"require"` | `"verify"` | `"require_and_verify"` (mTLS)

## transport/grpcx — gRPC 工具包

```go
import "github.com/Tsukikage7/servex/v2/transport/grpcx"
```

### 流包装（ServerStream context 替换）

```go
func myStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
    ctx := context.WithValue(ss.Context(), myKey, myValue)
    return handler(srv, grpcx.WrapServerStream(ss, ctx))
}
```

### Metadata 操作

```go
// 读取入站 metadata
traceID := grpcx.GetMetadataValue(ctx, "x-trace-id")
values := grpcx.GetMetadataValues(ctx, "x-roles")

// 设置出站 metadata（客户端调用前）
ctx = grpcx.AppendOutgoingMetadata(ctx, "x-trace-id", traceID)
ctx = grpcx.SetOutgoingMetadata(ctx, "x-trace-id", traceID)  // 替换已有

// 代理/网关场景：复制入站到出站
ctx = grpcx.CopyIncomingToOutgoing(ctx, "x-trace-id", "x-request-id")
```

### 错误处理

```go
// 便捷构造器
err := grpcx.NotFound("用户不存在")
err := grpcx.InvalidArgument("参数格式错误")
err := grpcx.PermissionDenied("权限不足")
err := grpcx.Unauthenticated("未登录")
err := grpcx.Internal("内部错误")
err := grpcx.Unavailable("服务不可用")
err := grpcx.AlreadyExists("资源已存在")
err := grpcx.DeadlineExceeded("请求超时")

// 通用构造
err := grpcx.Error(codes.FailedPrecondition, "前置条件不满足")
err := grpcx.Errorf(codes.InvalidArgument, "字段 %s 不合法", field)

// 检查与提取
if grpcx.IsCode(err, codes.NotFound) { ... }
code := grpcx.Code(err)     // codes.NotFound
msg  := grpcx.Message(err)  // "用户不存在"
```

### 健康检查

```go
// 标准 gRPC 健康检查（grpc.health.v1）
if err := grpcx.HealthCheck(ctx, conn); err != nil {
    log.Fatalf("服务不可用: %v", err)
}

// 等待连接就绪（带超时）
if err := grpcx.WaitForReady(ctx, conn, 5*time.Second); err != nil {
    log.Fatalf("连接超时: %v", err)
}
```

**关键 API：**
- `grpcx.WrapServerStream(stream, ctx)` — 替换 ServerStream 的 context（流式拦截器必备）
- `grpcx.GetMetadataValue(ctx, key)` / `GetMetadataValues` — 读取入站 metadata
- `grpcx.AppendOutgoingMetadata(ctx, kv...)` / `SetOutgoingMetadata` — 写出站 metadata
- `grpcx.CopyIncomingToOutgoing(ctx, keys...)` — 入站 → 出站透传（代理/网关场景）
- 错误便捷构造：`NotFound`、`InvalidArgument`、`PermissionDenied`、`Unauthenticated`、`Internal`、`Unavailable`、`AlreadyExists`、`DeadlineExceeded`
- `grpcx.IsCode(err, code)` / `Code(err)` / `Message(err)` — 错误检查与提取
- `grpcx.HealthCheck(ctx, conn)` / `WaitForReady(ctx, conn, timeout)` — 健康检查

## transport/debug — 调试面板

```go
import "github.com/Tsukikage7/servex/v2/transport/debug"

// 创建调试面板 handler
handler := debug.Handler(
    debug.WithRoutes(router),          // 注入路由表（显示已注册路由）
    debug.WithConfig(configData),      // 注入配置信息（脱敏后显示）
)

// 注册路由（建议仅开发/内网环境启用）
debug.RegisterRoutes(mux, handler)
// 注册后可访问：
//   /debug/routes  — 已注册路由列表
//   /debug/config  — 当前配置信息
//   /debug/health  — 健康检查汇总
//   /debug/metrics — 关键指标快照
//   /debug/build   — 构建信息（版本/提交/时间）
```

**关键 API：**
- `debug.Handler(opts...) http.Handler` — 创建调试面板处理器
- `debug.RegisterRoutes(mux, handler)` — 注册所有调试端点到 mux
- `debug.WithRoutes(router)` — 注入路由表信息
- `debug.WithConfig(data)` — 注入配置数据（建议脱敏敏感字段）
- 端点：`/debug/routes`、`/debug/config`、`/debug/health`、`/debug/metrics`、`/debug/build`

## transport/botserver — 平台无关的 Bot 框架（v2.0.1+）

`botserver` 提供平台无关的 Bot 接口、命令路由器、中间件链和状态存储。
具体平台实现在子包 `telegram` 和 `discord` 中。

### 核心接口

```go
import "github.com/Tsukikage7/servex/v2/transport/botserver"

// Bot 平台无关接口
type Bot interface {
    Handle(pattern string, handler HandlerFunc, middlewares ...Middleware)
    Use(middlewares ...Middleware)
    Start(ctx context.Context) error
    Stop() error
}

// Context 每条消息/命令的处理上下文
type Context interface {
    ChatID() string
    UserID() string
    Text() string
    Command() string   // "/start" -> "start"，非命令返回 ""
    Args() []string    // 命令参数，非命令返回 nil
    State() string
    SetState(state string)
    Reply(text string, opts ...ReplyOption) error
    Native() any       // 平台原始对象
}

// HandlerFunc 处理函数
type HandlerFunc func(ctx Context) error

// Middleware 中间件
type Middleware func(next HandlerFunc) HandlerFunc
```

### Router — 命令路由器

```go
router := botserver.NewRouter()

// 注册命令（匹配优先级：精确 > 通配符 "*" > 忽略）
router.Handle("start", startHandler)
router.Handle("help", helpHandler)
router.Handle("*", fallbackHandler)  // 通配符，匹配所有未命中的命令

// 全局中间件
router.Use(loggingMiddleware)

// 自定义错误处理
router.SetErrorHandler(func(ctx botserver.Context, err error) {
    log.Printf("错误 [chat=%s]: %v", ctx.ChatID(), err)
})

// 分发消息
router.Dispatch(ctx)
```

### StateStore — 对话状态存储

```go
// 内存存储（开发/单机场景）
store := botserver.NewMemoryStateStore()

// Redis 存储（生产多实例场景）
store := botserver.NewRedisStateStore(redisClient,
    botserver.WithKeyPrefix("mybot:state:"),  // 默认 "botstate:"
)
```

**关键类型：**
- `botserver.Bot` — 平台无关接口
- `botserver.Context` — 消息处理上下文（ChatID/UserID/Text/Command/Args/State/Reply/Native）
- `botserver.HandlerFunc` / `botserver.Middleware` — handler 和中间件类型
- `botserver.Router` — 命令路由器（`NewRouter`, `Handle`, `Use`, `Dispatch`, `SetErrorHandler`）
- `botserver.StateStore` — 状态存储接口（`Get`/`Set`/`Del`）
- `botserver.NewMemoryStateStore()` — 内存状态存储
- `botserver.NewRedisStateStore(client, opts...)` — Redis 状态存储
- `botserver.WithKeyPrefix(prefix)` — Redis key 前缀选项

## transport/botserver/telegram — Telegram Bot（Webhook 模式）

```go
import "github.com/Tsukikage7/servex/v2/transport/botserver/telegram"

// 创建 Telegram Bot
bot, err := telegram.New("YOUR_BOT_TOKEN",
    telegram.WithWebhookURL("https://example.com/bot/telegram"),
    telegram.WithWebhookPath("/bot/telegram"),       // 默认 "/bot/telegram"
    telegram.WithHTTPServer(router),                  // 注册 webhook 路由到现有 httpserver Router
    telegram.WithStateStore(redisStore),              // 默认 MemoryStateStore
    telegram.WithErrorHandler(func(ctx botserver.Context, err error) {
        log.Printf("错误: %v", err)
    }),
)
if err != nil { ... }

// 注册命令
bot.Handle("start", func(ctx botserver.Context) error {
    return ctx.Reply("欢迎使用！")
})

bot.Handle("echo", func(ctx botserver.Context) error {
    args := ctx.Args()
    if len(args) == 0 {
        return ctx.Reply("用法: /echo <消息>")
    }
    return ctx.Reply(strings.Join(args, " "))
})

// 通配符：处理所有非命令消息
bot.Handle("*", func(ctx botserver.Context) error {
    return ctx.Reply("未知命令，发送 /help 查看帮助")
})

// 全局中间件
bot.Use(loggingMiddleware)

// 启动（设置 Webhook，注册 HTTP 路由，非阻塞）
if err := bot.Start(ctx); err != nil { ... }
defer bot.Stop()

// 复用底层 BotAPI 客户端（如传给 notify/telegram）
tgSender := notifytelegram.NewSenderWithClient(bot.Client())
```

**关键选项：**
- `telegram.New(token, opts...)` — 创建 TelegramBot
- `WithWebhookURL(url)` — 公网 HTTPS Webhook URL，Start 时调用 SetWebhook
- `WithWebhookPath(path)` — webhook 路由路径（默认 "/bot/telegram"）
- `WithHTTPServer(router)` — 将 webhook 路由注册到现有 httpserver Router
- `WithStateStore(store)` — 对话状态存储（默认 MemoryStateStore）
- `WithErrorHandler(fn)` — handler 错误处理函数
- `bot.Client() *tgbotapi.BotAPI` — 获取底层客户端（可复用给 notify/telegram）

## transport/botserver/discord — Discord Bot（Gateway 模式）

```go
import "github.com/Tsukikage7/servex/v2/transport/botserver/discord"

// 创建 Discord Bot（内部自动添加 "Bot " 前缀）
bot, err := discord.New("YOUR_BOT_TOKEN",
    discord.WithStateStore(redisStore),
    discord.WithCommandPrefix("/"),                    // 默认 "/"
    discord.WithIntents(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages),
    discord.WithErrorHandler(func(ctx botserver.Context, err error) {
        log.Printf("错误: %v", err)
    }),
)
if err != nil { ... }

// 注册命令
bot.Handle("ping", func(ctx botserver.Context) error {
    return ctx.Reply("pong!")
})

bot.Handle("greet", func(ctx botserver.Context) error {
    return ctx.Reply("你好, " + ctx.UserID())
})

// 启动（建立 Gateway 连接，阻塞直到 ctx 取消）
if err := bot.Start(ctx); err != nil { ... }

// 复用底层 Session（如传给 notify/discord）
dcSender := notifydiscord.NewSenderWithClient(bot.Session())
```

**关键选项：**
- `discord.New(token, opts...)` — 创建 DiscordBot
- `WithStateStore(store)` — 对话状态存储（默认 MemoryStateStore）
- `WithCommandPrefix(prefix)` — 消息命令前缀（默认 "/"）
- `WithIntents(intents)` — 覆盖 Gateway Intents（默认 GuildMessages + DirectMessages + MessageContent）
- `WithErrorHandler(fn)` — handler 错误处理函数
- `bot.Session() *discordgo.Session` — 获取底层 Session（可复用给 notify/discord）

## transport/botserver/bottest — Bot 测试工具

```go
import "github.com/Tsukikage7/servex/v2/transport/botserver/bottest"

func TestBotHandler(t *testing.T) {
    // 创建测试 Bot 和消息记录器
    bot, recorder := bottest.NewTestBot()

    // 注册 handler（与真实 Bot 相同的 API）
    bot.Handle("ping", func(ctx botserver.Context) error {
        return ctx.Reply("pong!")
    })

    bot.Handle("echo", func(ctx botserver.Context) error {
        return ctx.Reply(strings.Join(ctx.Args(), " "))
    })

    // 模拟发送命令
    err := bot.Dispatch("/ping")
    require.NoError(t, err)
    assert.Equal(t, "pong!", recorder.Messages[0].Text)

    // 带参数的命令
    err = bot.Dispatch("/echo hello world")
    require.NoError(t, err)
    assert.Equal(t, "hello world", recorder.Messages[1].Text)

    // 自定义 chatID/userID
    err = bot.Dispatch("/ping",
        bottest.WithChatID("chat-123"),
        bottest.WithUserID("user-456"),
    )
    require.NoError(t, err)
    assert.Equal(t, "chat-123", recorder.Messages[2].ChatID)
}
```

**关键类型：**
- `bottest.NewTestBot() (*TestBot, *Recorder)` — 创建测试 Bot 和消息记录器
- `bottest.TestBot` — 实现 `botserver.Bot` 接口，Start/Stop 为空操作
- `bottest.Recorder` — 记录所有 Reply 调用，`Messages []RecordedMessage`
- `bottest.RecordedMessage` — 记录条目（`ChatID string`, `Text string`）
- `bot.Dispatch(text, opts...)` — 模拟一条入站消息/命令
- `bottest.WithChatID(id)` — 设置会话 ID（默认 "test-chat"）
- `bottest.WithUserID(id)` — 设置用户 ID（默认 "test-user"）
