# servex 中间件

**组合顺序：** waf → logging → tracing → metrics → ratelimit → circuitbreaker → retry → timeout → recovery

（logging 在 tracing 之前是 servex 约定：tracing 中间件将 trace ID 写入 context，logging 在其后可提取并输出到日志）

## circuitbreaker + ratelimit 组合示例

```go
// 熔断器：连续 5 次失败打开，30s 后进入半开尝试恢复
cb := circuitbreaker.New(
    circuitbreaker.WithFailureThreshold(5),
    circuitbreaker.WithSuccessThreshold(2),
    circuitbreaker.WithOpenTimeout(30 * time.Second),
)

// 令牌桶：100 req/s，桶容量 200（允许瞬时突发）
limiter := ratelimit.NewTokenBucket(100, 200)

// 注意中间件顺序：ratelimit 在 circuitbreaker 之前
srv := httpserver.New(mux,
    httpserver.WithLogger(log),
    httpserver.WithMiddlewares(
        ratelimit.HTTPMiddleware(limiter),
        circuitbreaker.HTTPMiddleware(cb),
    ),
)
```

完整示例：`docs/superpowers/examples/middleware/main.go`

## circuitbreaker — 熔断器

**关键选项：**
- `WithFailureThreshold(n)` — 连续失败 n 次后打开
- `WithSuccessThreshold(n)` — 半开状态成功 n 次后关闭
- `WithOpenTimeout(d)` — Open 状态持续时间，之后进入 HalfOpen

**集成方式：**
- `circuitbreaker.HTTPMiddleware(cb)` — HTTP 中间件（返回 503）
- `circuitbreaker.EndpointMiddleware(cb)` — endpoint 层中间件
- `cb.Execute(ctx, fn)` — 手动执行，自定义错误处理

## ratelimit — 限流

```go
// 令牌桶：平滑限流，允许瞬时突发
limiter := ratelimit.NewTokenBucket(rate, capacity)

// 滑动窗口：精确计数
limiter := ratelimit.NewSlidingWindow(limit, window)

// 固定窗口：性能最好
limiter := ratelimit.NewFixedWindow(limit, window)

// HTTP 中间件（超限返回 429）
ratelimit.HTTPMiddleware(limiter)

// Endpoint 中间件
ratelimit.EndpointMiddleware(limiter)
```

## retry — 重试

```go
// 指数退避重试，最多 3 次，基础间隔 100ms
mw := retry.New(
    retry.WithMaxAttempts(3),
    retry.WithBackoff(retry.ExponentialBackoff(100*time.Millisecond)),
    retry.WithRetryOn(func(err error) bool {
        return errors.Is(err, io.ErrTemporary)
    }),
)
```

## timeout — 超时控制

```go
mw := timeout.New(timeout.WithTimeout(5 * time.Second))
// 超时后返回 504，并取消下游 context
```

## cors — 跨域

```go
mw := cors.New(
    cors.WithAllowOrigins("https://example.com", "https://app.example.com"),
    cors.WithAllowMethods("GET", "POST", "PUT", "DELETE"),
    cors.WithAllowHeaders("Authorization", "Content-Type"),
    cors.WithMaxAge(86400),
)
```

## logging — 结构化日志

```go
// HTTP 访问日志
mw := logging.NewHTTP(log, logging.WithSkipPaths("/healthz", "/metrics"))

// gRPC 访问日志
mw := logging.NewGRPC(log)
```

## idempotency — 幂等性

```go
// 基于请求 ID 去重，需要 Store 实现（Redis 或内存）
mw := idempotency.New(
    idempotency.WithStore(redisStore),
    idempotency.WithTTL(24 * time.Hour),
)
```

## semaphore — 并发控制

```go
// 最多 100 个并发请求，超出返回 503
mw := semaphore.New(semaphore.WithLimit(100))
```

## secure — 安全头

```go
import "github.com/Tsukikage7/servex/middleware/secure"

// 使用默认配置（生产推荐）：自动设置 X-Frame-Options、HSTS、X-Content-Type-Options 等
mw := secure.HTTPMiddleware(nil)

// 自定义配置
mw := secure.HTTPMiddleware(&secure.Config{
    XFrameOptions:         "SAMEORIGIN",
    HSTSMaxAge:            63072000,        // 2 年
    HSTSIncludeSubdomains: true,
    HSTSPreload:           true,
    ContentSecurityPolicy: "default-src 'self'",
    IsDevelopment:         false,           // true 时跳过 HSTS（本地开发用）
})

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(secure.HTTPMiddleware(nil)),
)
```

**默认设置的头部：**
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `Referrer-Policy: strict-origin-when-cross-origin`

## csrf — CSRF 防护

```go
import "github.com/Tsukikage7/servex/middleware/csrf"

// 使用默认配置（Double Submit Cookie 模式）
mw := csrf.HTTPMiddleware(nil)

// 自定义配置
mw := csrf.HTTPMiddleware(&csrf.Config{
    CookieName:   "_csrf",
    HeaderName:   "X-CSRF-Token",   // 前端通过此 header 回传 token
    FormField:    "csrf_token",      // 或表单字段
    CookieMaxAge: 12 * time.Hour,
    Secure:       true,
    SameSite:     http.SameSiteStrictMode,
    Skipper: func(r *http.Request) bool {
        return strings.HasPrefix(r.URL.Path, "/webhook") // 跳过 webhook 回调
    },
})

// 在 handler 中读取 token（用于渲染到页面或 JSON 响应）
func myHandler(w http.ResponseWriter, r *http.Request) {
    token := csrf.TokenFromContext(r.Context())
    json.NewEncoder(w).Encode(map[string]string{"csrf_token": token})
}
```

**工作流程：**
1. GET 请求 → 生成 token → 写入 `_csrf` cookie → 注入 context
2. POST/PUT/DELETE → 读取 `_csrf` cookie + `X-CSRF-Token` header → 恒定时间比较

## bodylimit — 请求体大小限制

```go
import "github.com/Tsukikage7/servex/middleware/bodylimit"

// 直接指定字节数（1 MB）
mw := bodylimit.HTTPMiddleware(1 << 20)

// 使用 ParseLimit 解析人类可读大小
limit, err := bodylimit.ParseLimit("10MB") // 支持 B/KB/MB/GB/TB
if err != nil {
    log.Fatal(err)
}
mw := bodylimit.HTTPMiddleware(limit)

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(
        bodylimit.HTTPMiddleware(1 << 20), // API 路由限制 1 MB
    ),
)
```

**超限响应：** 返回 `413 Request Entity Too Large`

**实现：** Content-Length 快速检查 + `http.MaxBytesReader` 兜底，防止绕过

## recovery — Panic 恢复

```go
// HTTP 中间件（panic 时返回 500）
httpMw := recovery.HTTPMiddleware(recovery.WithLogger(log))

// gRPC 拦截器（panic 时返回 codes.Internal）
unaryInterceptor := recovery.UnaryServerInterceptor(recovery.WithLogger(log))
streamInterceptor := recovery.StreamServerInterceptor(recovery.WithLogger(log))

// Endpoint 中间件
endpointMw := recovery.EndpointMiddleware(recovery.WithLogger(log))

// 自定义 panic 处理
mw := recovery.HTTPMiddleware(
    recovery.WithLogger(log),
    recovery.WithHandler(func(ctx any, p any, stack []byte) error {
        // 自定义处理逻辑
        return fmt.Errorf("panic recovered: %v", p)
    }),
    recovery.WithStackSize(64 * 1024), // 堆栈大小，默认 64KB
)
```

**关键选项：**
- `recovery.WithLogger(log)` — 日志记录器（必需）
- `recovery.WithHandler(fn)` — 自定义 panic 处理函数
- `recovery.WithStackSize(n)` — 堆栈捕获大小
- 支持三种集成：`HTTPMiddleware`、`UnaryServerInterceptor`/`StreamServerInterceptor`、`EndpointMiddleware`

## signature — HMAC 请求签名

```go
// 服务端验签中间件
cfg := signature.DefaultConfig("shared-secret")
// 自定义
cfg = &signature.Config{
    Secret:          "shared-secret",
    HeaderName:      "X-Signature",    // 默认
    TimestampHeader: "X-Timestamp",    // 默认
    MaxAge:          5 * time.Minute,  // 防重放窗口
    Algorithm:       "sha256",         // "sha256" 或 "sha512"
}
handler = signature.HTTPMiddleware(cfg)(handler)
```

```go
// 客户端签名（自动设置 X-Timestamp + X-Signature header）
req, _ := http.NewRequest("POST", url, body)
_ = signature.SignRequest(req, "shared-secret")
// 或使用自定义配置
_ = signature.SignRequestWithConfig(req, cfg)
```

**签名算法：** `HMAC-SHA256(secret, timestamp + "." + body)`

**错误：** `ErrMissingSignature` / `ErrMissingTimestamp` / `ErrExpiredTimestamp` / `ErrInvalidSignature`（均返回 401）

**低级 API：**
- `signature.Sign(body, timestamp, secret)` — 计算签名
- `signature.Verify(body, timestamp, sig, secret)` — 常量时间比较验证

## gzip — 响应 gzip 压缩

```go
import "github.com/Tsukikage7/servex/middleware/gzip"

// 使用默认配置（压缩级别: DefaultCompression, 最小字节数: 256）
mw := gzip.New()

// 自定义配置
mw := gzip.New(
    gzip.WithLevel(gzip.BestSpeed),                             // 压缩级别（-1 到 9）
    gzip.WithMinLength(1024),                                   // 触发压缩的最小响应体字节数
    gzip.WithExcludePaths("/healthz", "/metrics"),              // 排除的路径前缀
    gzip.WithExcludeContentTypes("image/png", "image/jpeg"),    // 排除的 Content-Type
)

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(gzip.New()),
)

// 便捷包装（直接返回 http.Handler）
handler := gzip.Handler(mux, gzip.WithMinLength(512))
```

**默认行为：**
- 检查客户端 `Accept-Encoding` 是否包含 gzip
- 响应体小于 `MinLength`（默认 256 字节）时不压缩
- 自动设置 `Content-Encoding: gzip` 和 `Vary: Accept-Encoding`
- 支持 `http.Flusher` 接口
- 使用 `sync.Pool` 复用 gzip.Writer，减少 GC 压力

**关键选项：**
- `WithLevel(level)` — 压缩级别，`gzip.NoCompression`(-1) 到 `gzip.BestCompression`(9)
- `WithMinLength(n)` — 触发压缩的最小响应体字节数（默认 256）
- `WithExcludePaths(paths...)` — 排除的路径前缀
- `WithExcludeContentTypes(types...)` — 排除的 Content-Type

## adaptive — 自适应限流与降级

```go
import "github.com/Tsukikage7/servex/middleware/adaptive"

// 基于 CPU 使用率限流
limiter, err := adaptive.New(&adaptive.Config{
    Strategy:       adaptive.StrategyCPU,
    CPUThreshold:   0.8,                     // CPU 使用率超 80% 触发
    WindowSize:     10 * time.Second,        // 指标采集窗口（默认 10s）
    CooldownPeriod: 5 * time.Second,         // 触发后冷却时间（默认 5s）
})

// 基于 P99 延迟限流
limiter, err := adaptive.New(&adaptive.Config{
    Strategy:         adaptive.StrategyLatency,
    LatencyThreshold: 500 * time.Millisecond,
})

// 基于错误率限流
limiter, err := adaptive.New(&adaptive.Config{
    Strategy:           adaptive.StrategyErrorRate,
    ErrorRateThreshold: 0.1,  // 错误率超 10% 触发
})

// 组合策略（任一条件触发）
limiter, err := adaptive.New(&adaptive.Config{
    Strategy:           adaptive.StrategyComposite,
    CPUThreshold:       0.8,
    LatencyThreshold:   500 * time.Millisecond,
    ErrorRateThreshold: 0.1,
    DegradeHandler:     degradeHandler,  // 降级处理器
}, adaptive.WithLogger(stdLogger))

// HTTP 中间件（超载返回 503）
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(limiter.Middleware()),
)

// gRPC 拦截器（超载返回 codes.ResourceExhausted）
grpcServer := grpc.NewServer(
    grpc.UnaryInterceptor(limiter.GRPCUnaryInterceptor()),
)

// 手动记录延迟和错误（用于 StrategyLatency/StrategyErrorRate）
limiter.RecordLatency(200 * time.Millisecond)
limiter.RecordError()
limiter.RecordSuccess()

// 获取限流器状态
status := limiter.Status()
fmt.Printf("限流中: %v, CPU: %.2f, P99: %v, 错误率: %.2f\n",
    status.IsLimiting, status.CurrentCPU, status.CurrentLatencyP99, status.CurrentErrorRate)
```

**策略类型：**
- `StrategyCPU` — 基于 CPU 使用率（goroutine 数/核估算）
- `StrategyLatency` — 基于 P99 延迟
- `StrategyErrorRate` — 基于错误率
- `StrategyComposite` — 组合多种策略，任一触发即限流

**关键类型：**
- `adaptive.Limiter` — 自适应限流器（`Allow`, `Middleware`, `GRPCUnaryInterceptor`, `RecordLatency`, `RecordError`, `RecordSuccess`, `Status`）
- `adaptive.Config` — 配置（`Strategy`, `CPUThreshold`, `LatencyThreshold`, `ErrorRateThreshold`, `WindowSize`, `CooldownPeriod`, `DegradeHandler`）
- `adaptive.Status` — 当前状态（`IsLimiting`, `CurrentCPU`, `CurrentLatencyP99`, `CurrentErrorRate`, `TotalRequests`, `DroppedRequests`）
- `adaptive.MetricsCollector` — 指标采集器接口（`OnAllow`, `OnDrop`）
- `WithLogger(l)` / `WithMetricsCollector(mc)` — 配置选项

## trace — 链路追踪增强

```go
import "github.com/Tsukikage7/servex/middleware/trace"
```

统一 trace-id 在日志、响应头、下游调用中的传播，构建于 `observability/tracing` 之上。

```go
// HTTP 中间件
mw := trace.HTTPMiddleware(nil) // 使用默认配置

// 自定义配置
mw := trace.HTTPMiddleware(&trace.Config{
    TraceIDHeader:    "X-Trace-ID",    // 默认
    RequestIDHeader:  "X-Request-ID",  // 默认
    Logger:           log,             // 自动注入 trace_id 字段到日志
})

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(
        trace.HTTPMiddleware(nil),
    ),
)
```

```go
// gRPC 拦截器
unaryInterceptor  := trace.GRPCUnaryInterceptor(nil)
streamInterceptor := trace.GRPCStreamInterceptor(nil)
```

```go
// 在 handler 中读取
traceID := trace.TraceIDFromContext(ctx)
reqID   := trace.RequestIDFromContext(ctx)

// 向下游传播（HTTP 客户端调用）
trace.InjectHTTPHeaders(ctx, req)

// 向下游传播（gRPC 客户端调用）
ctx = trace.InjectGRPCMetadata(ctx)
```

**默认行为：**
1. 从请求头（`X-Trace-ID`）提取 trace-id，不存在则生成 UUID
2. 从请求头（`X-Request-ID`）提取 request-id，不存在则生成 UUID
3. 将 trace-id / request-id 写入响应头
4. 注入 logger context（后续 `log.Info(ctx, ...)` 自动携带 `trace_id` 字段）

**关键 API：**
- `trace.HTTPMiddleware(cfg)` — HTTP 中间件
- `trace.GRPCUnaryInterceptor(cfg)` / `GRPCStreamInterceptor(cfg)` — gRPC 拦截器
- `trace.TraceIDFromContext(ctx)` / `RequestIDFromContext(ctx)` — 读取 context
- `trace.InjectHTTPHeaders(ctx, req)` — 传播到下游 HTTP 请求
- `trace.InjectGRPCMetadata(ctx)` — 传播到下游 gRPC 调用
- `trace.DefaultConfig()` — 默认配置（TraceIDHeader: `X-Trace-ID`，RequestIDHeader: `X-Request-ID`）

## waf — Web 应用防火墙

```go
import "github.com/Tsukikage7/servex/middleware/waf"

// 使用默认规则集（SQL 注入/XSS/路径遍历/命令注入）
mw := waf.New()

// 自定义配置
mw := waf.New(
    waf.WithRuleSet(waf.CoreRuleSet),          // 核心规则集
    waf.WithMode(waf.ModeBlock),               // Block（拦截） 或 Detect（仅记录）
    waf.WithCustomRules(myRules...),           // 追加自定义规则
    waf.WithExcludePaths("/healthz", "/metrics"), // 排除路径
    waf.WithLogger(log),                       // 日志记录器
)

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(waf.New()),
)
```

**内置检测规则：**
- SQL 注入（`UNION SELECT`、`OR 1=1`、注释闭合等）
- XSS（`<script>`、`onerror=`、`javascript:` 等）
- 路径遍历（`../`、`..%2f` 等）
- 命令注入（`;`、`|`、`$()`、反引号等）

**关键选项：**
- `WithRuleSet(rs)` — 规则集（`CoreRuleSet` 为默认）
- `WithMode(mode)` — `ModeBlock`（拦截并返回 403）或 `ModeDetect`（仅记录，不拦截）
- `WithCustomRules(rules...)` — 追加自定义检测规则
- `WithExcludePaths(paths...)` — 排除的路径前缀
- `WithLogger(log)` — 日志记录器

## version — API 版本化

```go
import "github.com/Tsukikage7/servex/middleware/version"

// 路径前缀模式：/v1/users、/v2/users
mw := version.New(
    version.WithPathPrefix("/v"),            // 从路径提取版本号
    version.WithDefaultVersion("1"),         // 默认版本
)

// Header 模式：Accept: application/vnd.api.v2+json
mw := version.New(
    version.WithHeader("Accept"),            // 从 Header 提取版本号
    version.WithDefaultVersion("1"),
)

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(mw),
)

// 在 handler 中读取版本号
func handler(w http.ResponseWriter, r *http.Request) {
    ver := version.FromContext(r.Context()) // "1", "2", ...
}
```

**关键选项：**
- `WithPathPrefix(prefix)` — 路径版本前缀（如 `/v`）
- `WithHeader(name)` — 从指定 Header 提取版本号
- `WithDefaultVersion(v)` — 默认版本号（未指定时使用）
- `version.FromContext(ctx)` — 从 context 获取当前版本号

## fallback — 优雅降级

```go
import "github.com/Tsukikage7/servex/middleware/fallback"

// 5xx 错误自动降级
mw := fallback.New(
    fallback.WithHandler(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
    }),
    fallback.WithStatusCodes(500, 502, 503),  // 触发降级的状态码
    fallback.WithTimeout(3 * time.Second),     // 上游超时也触发降级
)

// 注入 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(mw),
)
```

**关键选项：**
- `WithHandler(h)` — 降级处理器（fallback 响应）
- `WithStatusCodes(codes...)` — 触发降级的 HTTP 状态码（默认 5xx）
- `WithTimeout(d)` — 上游超时阈值，超时后自动降级

## loadshed — 负载卸载

```go
import "github.com/Tsukikage7/servex/middleware/loadshed"

// 基于并发数限制
mw := loadshed.New(
    loadshed.WithMaxConcurrent(500),           // 最大并发请求数
    loadshed.WithMaxQueueDepth(100),           // 等待队列深度
    loadshed.WithLatencyThreshold(2 * time.Second), // P99 延迟阈值
)

// 注入 httpserver（超载返回 503）
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(mw),
)
```

**关键选项：**
- `WithMaxConcurrent(n)` — 最大同时处理请求数，超出直接返回 503
- `WithMaxQueueDepth(n)` — 等待队列最大深度
- `WithLatencyThreshold(d)` — 延迟阈值，超过后开始卸载新请求
- `WithOnShed(fn)` — 卸载回调（记录日志/指标）
