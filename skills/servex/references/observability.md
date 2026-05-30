# servex 可观测性

## observability/metrics — Prometheus + OpenTelemetry 指标

```go
// MustNewMetrics 初始化失败直接 panic（适合 main 函数）
cfg := metrics.DefaultConfig()
cfg.ServiceName = "my-service"
cfg.Version = "v2.0.5"
m := metrics.MustNewMetrics(cfg)

// NewMetrics 返回 error
m, err := metrics.NewMetrics(cfg)
if err != nil { ... }

// HTTP 中间件（自动记录请求数、延迟、状态码）
mux.Handle("/metrics", promhttp.Handler())
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(m.HTTPMiddleware()),
)
```

### OpenTelemetry Metrics（OTel）

```go
import "github.com/Tsukikage7/servex/v2/observability/metrics"

// 创建 OTel Metrics 收集器
otelMetrics, err := metrics.NewOTel(
    metrics.WithMeterProvider(meterProvider),  // 自定义 MeterProvider
    metrics.WithExporter(exporter),            // OTLP / Prometheus 导出器
)
if err != nil { ... }

// 与 Prometheus 共存（双后端）
m := metrics.MustNewMetrics(metrics.DefaultConfig())
m.EnableOTel(otelMetrics)

// HTTP 中间件（同时写入 Prometheus 和 OTel）
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(m.HTTPMiddleware()),
)
```

### Config 结构体（v2.0.5+）

```go
type Config struct {
    Path        string // 指标暴露路径，默认 /metrics
    Namespace   string // 指标命名空间
    ServiceName string // 服务名称，用于 service_info 指标
    Version     string // 服务版本，用于 service_info 指标
}
```

当 `ServiceName` 或 `Version` 不为空时，自动注册 `service_info` Gauge 指标：

```
# HELP service_info 服务元信息
# TYPE service_info gauge
service_info{service_name="my-service",version="v2.0.5"} 1
```

```go
cfg := &metrics.Config{
    Path:        "/metrics",
    Namespace:   "app",
    ServiceName: "order",
    Version:     "v2.0.5",
}
m := metrics.MustNewMetrics(cfg)
```

**关键选项：**
- `metrics.DefaultConfig()` — 默认配置（Path="/metrics", Namespace="app"）
- `metrics.Config{ServiceName, Version}` — 设置服务名称和版本，自动注册 `service_info` gauge（v2.0.5+）
- `metrics.NewOTel(opts...)` — 创建 OTel Metrics 收集器
- `WithMeterProvider(mp)` — 自定义 OpenTelemetry MeterProvider
- `WithServiceName(name)` — OTel Meter 的 instrumentation scope 服务名
- `WithExporter(exp)` — 指标导出器（OTLP HTTP/gRPC、Prometheus Remote Write）
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

// 从 context 取出 logger（trace 中间件已注入 traceId/spanId）
logger.FromContext(ctx).Info("带链路追踪的日志")
```

**Logger 接口方法：**
- 级别方法：`Debug`/`Info`/`Warn`/`Error`/`Fatal`/`Panic`（及 `f` 格式化版本）
- `With(fields...) Logger` — 附加结构化字段
- `Sync() error` / `Close() error` — 刷新/关闭

**Context 集成函数：**
- `logger.NewContext(ctx, l)` — 将 logger 存入 context（中间件层调用）
- `logger.FromContext(ctx)` — 从 context 取出 logger（业务层调用）
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
import "github.com/Tsukikage7/servex/v2/observability/slo"

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

## observability/alerting — 告警规则引擎

```go
import "github.com/Tsukikage7/servex/v2/observability/alerting"

// 创建指标提供者（实现 MetricProvider 接口）
type myProvider struct{}
func (p *myProvider) Query(ctx context.Context, metric string) (float64, error) {
    // 从 Prometheus/自定义数据源查询指标值
    return getCurrentValue(metric)
}

// 创建告警引擎
engine := alerting.New(&myProvider{},
    alerting.WithDefaultEvalInterval(15*time.Second),  // 默认评估间隔
    alerting.WithNotifier(myNotifier),                  // 告警通知器
    alerting.WithHistorySize(1000),                     // 历史记录容量
    alerting.WithLogger(log.Printf),                    // 日志记录器
)

// 添加阈值告警规则
engine.AddRule(&alerting.Rule{
    ID:   "high-cpu",
    Name: "CPU 使用率过高",
    Type: alerting.RuleThreshold,
    Condition: alerting.Condition{
        Metric:    "cpu_usage_percent",
        Operator:  alerting.OpGT,
        Threshold: 80,
    },
    For:          5 * time.Minute, // Pending 持续 5 分钟后 Firing
    EvalInterval: 15 * time.Second,
    Labels:       map[string]string{"severity": "critical", "team": "infra"},
    Annotations:  map[string]string{"summary": "CPU 使用率超过 80%"},
})

// 添加速率告警规则
engine.AddRule(&alerting.Rule{
    ID:   "high-error-rate",
    Name: "错误率过高",
    Type: alerting.RuleRate,
    Condition: alerting.Condition{
        Metric:    "http_error_rate",
        Operator:  alerting.OpGT,
        Threshold: 0.05,
    },
    For: 2 * time.Minute,
})

// 添加缺失检测规则
engine.AddRule(&alerting.Rule{
    ID:   "heartbeat-missing",
    Name: "心跳缺失",
    Type: alerting.RuleAbsence,
    Condition: alerting.Condition{
        Metric:   "service_heartbeat",
        Operator: alerting.OpGT,
        Threshold: 0,
    },
    For: time.Minute,
})

// 启动评估循环
engine.Start(ctx)
defer engine.Stop(ctx)

// 一次性评估（用于测试或手动触发）
alerts, err := engine.Evaluate(ctx)

// 查看活跃告警
active := engine.ActiveAlerts()

// 查看告警历史
history := engine.AlertHistory(50)

// 规则 CRUD
rules := engine.ListRules()
rule, _ := engine.GetRule("high-cpu")
engine.RemoveRule("high-cpu")
```

**关键类型：**
- `alerting.Engine` — 告警引擎（`AddRule`, `RemoveRule`, `GetRule`, `ListRules`, `Start`, `Stop`, `Evaluate`, `ActiveAlerts`, `AlertHistory`）
- `alerting.Rule` — 规则定义（`ID`, `Name`, `Type`, `Condition`, `Labels`, `Annotations`, `EvalInterval`, `For`）
- `alerting.Alert` — 告警实例（`ID`, `RuleID`, `State`, `Value`, `Labels`, `Annotations`, `StartsAt`, `EndsAt`, `UpdatedAt`）
- `alerting.Condition` — 告警条件（`Metric`, `Operator`, `Threshold`, `Duration`）
- `alerting.MetricProvider` — 指标提供者接口（`Query(ctx, metric) (float64, error)`）
- `alerting.Notifier` — 通知接口（`Notify(ctx, alert) error`）
- `alerting.New(provider, opts...)` — 创建引擎

**告警状态：** `StateOK`, `StatePending`, `StateFiring`, `StateResolved`

**规则类型：** `RuleThreshold`（阈值）, `RuleRate`（速率）, `RuleAbsence`（缺失检测）

**运算符：** `OpGT`, `OpGTE`, `OpLT`, `OpLTE`, `OpEQ`, `OpNEQ`

**选项：**
- `WithLogger(printf)` — 日志记录器
- `WithNotifier(n)` — 告警通知器
- `WithDefaultEvalInterval(d)` — 默认评估间隔（默认 15s）
- `WithHistorySize(n)` — 历史记录容量（默认 1000）

**错误：** `ErrNilProvider`, `ErrRuleNotFound`, `ErrDuplicateRule`, `ErrInvalidCondition`, `ErrAlreadyRunning`, `ErrNotRunning`

**状态转换：** OK → Pending（条件满足）→ Firing（超过 `For` 时间）→ Resolved（条件恢复）；Pending → OK（条件不再满足，回退）

## observability/profiling — 持续性能剖析

```go
import "github.com/Tsukikage7/servex/v2/observability/profiling"

// 默认配置（CPU/Heap/Goroutine，60s 间隔，10s CPU 采样时长）
cfg := profiling.DefaultConfig()
cfg.Labels = map[string]string{"service": "my-service", "env": "prod"}

// 创建剖析器
p, err := profiling.New(cfg,
    profiling.WithLogger(log.Printf),
    profiling.WithExporter(profiling.NewFileExporter("./profiles")),  // 保存到本地
    profiling.WithHTTPPrefix("/debug/pprof"),                        // pprof HTTP 前缀
)
if err != nil { ... }

// 启动周期采集
if err := p.Start(ctx); err != nil { ... }
defer p.Stop(ctx)

// 单次采集
prof, err := p.Collect(ctx, profiling.ProfileHeap)
fmt.Printf("type=%s, size=%d bytes\n", prof.Type, len(prof.Data))

// 挂载 pprof HTTP 端点
mux.Handle("/debug/pprof/", p.Handler())

// 查看状态
status := p.Status()
fmt.Printf("running=%v, collected=%d, errors=%d\n",
    status.Running, status.CollectedCount, status.ErrorCount)
```

**关键类型：**
- `profiling.Profiler` — 剖析器（`Start`, `Stop`, `Collect`, `Handler`, `Status`）
- `profiling.Config` — 配置（`Enabled`, `Types`, `Interval`, `Duration`, `OutputDir`, `Labels`）
- `profiling.Profile` — 采集结果（`Type`, `Data`, `Timestamp`, `Duration`, `Labels`）
- `profiling.Status` — 运行状态（`Running`, `LastCollected`, `CollectedCount`, `ErrorCount`, `ActiveProfiles`）
- `profiling.Exporter` — 导出接口（`Export(ctx, *Profile) error`）
- `profiling.FileExporter` — 文件导出器（保存为 `{type}_{timestamp}.pprof`）
- `profiling.DefaultConfig()` — 默认配置

**剖析类型：**
- `ProfileCPU` / `ProfileHeap` / `ProfileGoroutine` / `ProfileBlock` / `ProfileMutex` / `ProfileAllocs` / `ProfileThreadCreate`

**选项：**
- `WithLogger(printf)` — 日志记录器
- `WithExporter(e)` — 自定义导出后端
- `WithHTTPPrefix(prefix)` — pprof HTTP 路径前缀（默认 "/debug/pprof"）

**错误：**
- `ErrNilConfig`, `ErrAlreadyRunning`, `ErrNotRunning`, `ErrInvalidProfileType`
