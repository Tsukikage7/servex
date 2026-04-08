# servex 可观测性

## observability/metrics — Prometheus 指标

```go
// MustNewMetrics 初始化失败直接 panic（适合 main 函数）
m := metrics.MustNewMetrics(metrics.DefaultConfig("my-service"))

// NewMetrics 返回 error
m, err := metrics.NewMetrics(metrics.DefaultConfig("my-service"))
if err != nil { ... }

// HTTP 中间件（自动记录请求数、延迟、状态码）
mux.Handle("/metrics", promhttp.Handler())
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(m.HTTPMiddleware()),
)
```

**关键选项：**
- `metrics.DefaultConfig(serviceName)` — 默认配置，注册 HTTP/gRPC 指标
- `m.HTTPMiddleware()` — `func(http.Handler) http.Handler`
- `m.GRPCUnaryInterceptor()` — gRPC 一元拦截器

## observability/tracing — OpenTelemetry 追踪

```go
// OTLP HTTP 导出（Jaeger、Grafana Tempo 等）
tracer, err := tracing.NewTracer(tracing.TracingConfig{
    ServiceName: "my-service",
    OTLP: &tracing.OTLPConfig{
        Endpoint: "http://localhost:4318", // OTLP HTTP 端口
    },
})
if err != nil { ... }
defer tracer.Shutdown(ctx)

// MustNewTracer 初始化失败直接 panic
tracer := tracing.MustNewTracer(tracing.TracingConfig{...})

// 与 httpserver 集成（自动注入 trace ID 到 context）
srv := httpserver.New(mux,
    httpserver.WithTrace("my-service"), // 快捷选项，内部使用默认 tracer
)
```

**与 logging 配合（组合顺序）：**

```
requestid → logging → tracing → metrics → ...
```

logging 在 tracing 之前：tracing 将 trace ID 写入 context，logging 可在后续请求处理中提取并输出。

## observability/logger — 结构化日志

```go
// 创建 logger（基于 zap）
log, err := logger.NewLogger(&logger.Config{
    Type:        logger.TypeZap,
    ServiceName: "my-service",
    Level:       logger.LevelInfo,     // debug/info/warn/error/fatal/panic
    Format:      logger.FormatJSON,    // json / console
    Output:      logger.OutputBoth,    // console / file / both
    LogDir:      "./logs",
    LevelSeparate: true,               // 按级别分文件
    RotationEnabled: true,
    RotationTime: logger.RotationDaily,
    MaxAge:      7,                    // 日志保留天数
    Compress:    true,
    EnableCaller: true,
    EnableStacktrace: false,
    TimeFormat:  logger.TimeFormatISO8601,
})
if err != nil { ... }
defer log.Close()

// MustNewLogger 失败时 panic
log := logger.MustNewLogger(&logger.Config{...})

// 基础日志
log.Info("服务启动")
log.Errorf("请求失败: %v", err)

// 结构化字段
log.With(
    logger.Field{Key: "user_id", Value: "u-1"},
    logger.Field{Key: "latency_ms", Value: 42},
).Info("请求完成")

// 注入 context（自动提取 traceId/spanId）
log.WithContext(ctx).Info("带链路追踪的日志")
```

**Logger 接口方法：**
- 级别方法：`Debug`/`Info`/`Warn`/`Error`/`Fatal`/`Panic`（及 `f` 格式化版本）
- `With(fields...) Logger` — 附加结构化字段
- `WithContext(ctx) Logger` — 注入 context（自动提取 traceId）
- `Sync() error` / `Close() error` — 刷新/关闭

**辅助函数：**
- `logger.ContextWithTraceID(ctx, traceID)` — 注入 traceId 到 context
- `logger.ContextWithSpanID(ctx, spanID)` — 注入 spanId 到 context

## observability/logshipper — 日志投递（ES/Kafka）

```go
// 创建 ES sink（按日分索引：logs-2026.04.05）
esSink := logshipper.NewElasticsearchSink(esClient,
    logshipper.WithIndexPrefix("logs-"),
    logshipper.WithDateSuffix("2006.01.02"),
)

// 创建 Kafka sink
kafkaSink := logshipper.NewKafkaSink(publisher,
    logshipper.WithTopic("app-logs"),
)

// 创建并启动 Shipper
s := logshipper.New(esSink,
    logshipper.WithBatchSize(200),
    logshipper.WithFlushInterval(3*time.Second),
    logshipper.WithBufferSize(20000),
    logshipper.WithDropOnFull(true),
    logshipper.WithErrorHandler(func(err error) { /* 告警/降级 */ }),
)
s.Start(ctx)
defer s.Close()
```

**Hook 集成（推荐）：**

```go
// 方式一：附加到 *zap.Logger（最常用）
zapLogger = logshipper.AttachToLogger(zapLogger, s)

// 方式二：手动组合 zapcore.Core
hook := logshipper.ZapHook(s)
zapLogger = zap.New(zapcore.NewTee(originalCore, hook))

// 方式三：包装 logger.Logger 接口（不直接持有 *zap.Logger 时）
hooked := logshipper.NewLoggerHook(innerLogger, s, "info")
// minLevel="info"：debug 日志不投递，info/warn/error/fatal/panic 才投递
hooked.Infof("用户登录: %s", userID)
```

**关键选项：**
- `WithBatchSize(n)` — 达到 n 条立即 flush（默认 100）
- `WithFlushInterval(d)` — 定时 flush 间隔（默认 5s）
- `WithBufferSize(n)` — 缓冲 channel 大小（默认 10000）
- `WithDropOnFull(true)` — 缓冲满时丢弃而非阻塞（默认 true）
- `WithErrorHandler(fn)` — 投递失败回调（默认 nop）
- `s.Flush(ctx)` — 主动阻塞刷新缓冲区

## observability/slo — SLO/SLI 追踪

```go
import "github.com/Tsukikage7/servex/observability/slo"

// 定义 SLO 目标
objectives := []*slo.Objective{
    {Name: "api_availability", Target: 0.999, Window: 30 * 24 * time.Hour, Description: "API 可用性 99.9%"},
    {Name: "latency_p99",     Target: 0.99,  Window: 7 * 24 * time.Hour,  Description: "P99 延迟达标率 99%"},
}

// 创建追踪器
tracker, err := slo.NewTracker(objectives,
    slo.WithCheckInterval(time.Minute),          // SLO 检查间隔（默认 1 分钟）
    slo.WithPrometheusNamespace("myapp"),         // Prometheus 指标命名空间（默认 "app"）
    slo.WithLogger(log.Printf),                  // 日志记录器
)
if err != nil { ... }

// 记录事件
tracker.Record(ctx, "api_availability", true)  // 好事件
tracker.Record(ctx, "api_availability", false) // 坏事件

// 查看状态
status, _ := tracker.Status("api_availability")
fmt.Printf("SLI: %.4f, 目标: %.3f, 错误预算剩余: %.2f%%, 消耗速率: %.2f, 违反: %v\n",
    status.SLIValue, status.Objective.Target,
    status.ErrorBudgetRemaining*100, status.BurnRate, status.IsBreaching)

// 获取所有目标状态
allStatuses := tracker.AllStatuses()

// 快速检查是否违反 SLO
if tracker.IsBreaching("api_availability") {
    alert("SLO 违反！")
}

// 注册 SLO 违反回调
tracker.OnBreach(func(status *slo.Status) {
    log.Printf("SLO 违反: %s, SLI=%.4f, 目标=%.3f",
        status.Objective.Name, status.SLIValue, status.Objective.Target)
})

// Prometheus 集成
prometheus.MustRegister(tracker.PrometheusCollector())
```

**关键类型：**
- `slo.Tracker` — SLO 追踪器（`Record`, `Status`, `AllStatuses`, `IsBreaching`, `OnBreach`, `PrometheusCollector`）
- `slo.Objective` — SLO 目标定义（`Name`, `Target`, `Window`, `Description`）
- `slo.Status` — 状态（`TotalEvents`, `GoodEvents`, `BadEvents`, `SLIValue`, `ErrorBudget`, `ErrorBudgetRemaining`, `BurnRate`, `IsBreaching`）
- `slo.NewTracker(objectives, opts...)` — 创建追踪器
- `WithCheckInterval(d)` — 检查间隔（默认 1 分钟）
- `WithPrometheusNamespace(ns)` — Prometheus 命名空间（默认 "app"）
- `WithLogger(printf)` — 日志记录器
- 错误：`ErrObjectiveNotFound`, `ErrInvalidTarget`, `ErrNilObjective`

**Prometheus 指标：**
- `{namespace}_slo_events_total{name, result}` — 总事件计数器
- `{namespace}_slo_error_budget_remaining{name}` — 剩余错误预算
- `{namespace}_slo_burn_rate{name}` — 错误预算消耗速率
