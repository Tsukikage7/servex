# servex v2.0.0 迁移指南

本文档面向从 servex v1.x 升级到 v2.0.0 的用户，涵盖所有破坏性变更及逐步迁移方法。

---

## 目录

1. [破坏性变更总览](#1-破坏性变更总览)
2. [逐步迁移](#2-逐步迁移)
3. [v2 新特性](#3-v2-新特性)
4. [配置变更](#4-配置变更)
5. [废弃项](#5-废弃项)

---

## 1. 破坏性变更总览

### 1.1 Logger: `WithContext` 移除，改用 `NewContext`/`FromContext`

v1 中 `Logger` 接口包含 `WithContext(ctx) Logger` 方法，在中间件和业务代码中需要手动传递。
v2 将 logger 的 context 管理改为包级函数，服务器自动注入，业务代码只需 `FromContext` 取出即可。

**v1 (旧)**
```go
// Logger 接口包含 WithContext
type Logger interface {
    WithContext(ctx context.Context) Logger
    Info(args ...any)
    // ...
}

// 业务代码
func handler(ctx context.Context, log logger.Logger) {
    l := log.WithContext(ctx)
    l.Info("处理请求")
}
```

**v2 (新)**
```go
// Logger 接口不再包含 WithContext
type Logger interface {
    Debug(args ...any)
    Info(args ...any)
    With(fields ...Field) Logger
    Sync() error
    Close() error
    // ...
}

// 包级函数
func NewContext(ctx context.Context, l Logger) context.Context  // 将 logger 存入 ctx
func FromContext(ctx context.Context) Logger                     // 从 ctx 取出 logger

// 业务代码 - 直接从 context 取
func handler(ctx context.Context) {
    logger.FromContext(ctx).Info("处理请求")
}
```

**迁移操作:**
1. 删除所有 `log.WithContext(ctx)` 调用
2. 业务代码统一改为 `logger.FromContext(ctx).Info(...)`
3. 如需将 logger 存入 context（通常在中间件中），使用 `logger.NewContext(ctx, log)`

### 1.2 HTTP/gRPC Server 内置中间件全部移除

v1 中 `httpserver`/`grpcserver`/`gateway` 内置了 recovery、logging、tracing、auth、metrics、ratelimit、tenant、clientip 等中间件选项。
v2 全部移除，改为用户通过 `WithMiddlewares()` 自行组合。

**以下 Option 已删除:**

| 包 | 已删除的 Option |
|---|---|
| `httpserver` | `WithRecovery()`, `WithLogging()`, `WithTracing()`, `WithAuth()`, `WithMetrics()`, `WithRateLimit()`, `WithTenant()`, `WithClientIP()` |
| `grpcserver` | `WithRecovery()`, `WithLogging()`, `WithTracing()`, `WithAuth()`, `WithMetrics()`, `WithRateLimit()`, `WithTenant()`, `WithClientIP()` |
| `gateway` | 同上 |

**v1 (旧)**
```go
srv := httpserver.New(mux,
    httpserver.WithLogger(log),
    httpserver.WithRecovery(),
    httpserver.WithLogging(),
    httpserver.WithTracing("my-service"),
    httpserver.WithAuth(authenticator),
    httpserver.WithMetrics(collector),
)
```

**v2 (新)**
```go
srv := httpserver.New(mux,
    httpserver.WithLogger(log),
    httpserver.WithMiddlewares(
        recovery.HTTPMiddleware(recovery.WithLogger(log)),
        logging.HTTPMiddleware(logging.WithLogger(log), logging.WithSkipPaths("/health")),
        trace.HTTPMiddleware(&trace.Config{Logger: log}),
        auth.HTTPMiddleware(authenticator),
        metrics.HTTPMiddleware(collector),
    ),
)
```

**gRPC 同理:**

```go
// v1 (旧)
srv := grpcserver.New(
    grpcserver.WithLogger(log),
    grpcserver.WithRecovery(),
    grpcserver.WithLogging(),
)

// v2 (新)
srv := grpcserver.New(
    grpcserver.WithLogger(log),
    grpcserver.WithUnaryInterceptor(
        recovery.GRPCUnaryInterceptor(recovery.WithLogger(log)),
        logging.GRPCUnaryInterceptor(logging.WithLogger(log)),
        trace.GRPCUnaryInterceptor(&trace.Config{Logger: log}),
    ),
    grpcserver.WithStreamInterceptor(
        recovery.GRPCStreamInterceptor(recovery.WithLogger(log)),
        logging.GRPCStreamInterceptor(logging.WithLogger(log)),
        trace.GRPCStreamInterceptor(&trace.Config{Logger: log}),
    ),
)
```

> **注意:** v2 中 `httpserver`/`grpcserver` 会自动将 logger 注入到每个请求的 context 中（最外层），无需手动添加 logger 注入中间件。

### 1.3 `middleware/requestid` 包删除

`middleware/requestid` 包已被完全删除，其功能由 `middleware/trace` 覆盖。

**v1 (旧)**
```go
import "github.com/Tsukikage7/servex/middleware/requestid"

httpserver.WithMiddlewares(
    requestid.HTTPMiddleware(),
)
```

**v2 (新)**
```go
import "github.com/Tsukikage7/servex/middleware/trace"

httpserver.WithMiddlewares(
    trace.HTTPMiddleware(&trace.Config{
        TraceIDHeader: "X-Trace-ID",
        Logger:        log,
    }),
)

// 获取 trace ID
traceID := trace.TraceIDFromContext(ctx)
```

### 1.4 `middleware/trace` 变更：移除 requestId，只管 traceId/spanId

v1 的 trace 中间件同时管理 requestId 和 traceId。v2 移除 requestId，专注 traceId/spanId 传播，并优先复用 OTel span。

**v1 (旧)**
```go
cfg := &trace.Config{
    TraceIDHeader:   "X-Trace-ID",
    RequestIDHeader: "X-Request-ID",  // v2 已删除
}
```

**v2 (新)**
```go
cfg := &trace.Config{
    TraceIDHeader:    "X-Trace-ID",
    PropagateHeaders: []string{"X-Custom-Header"},
    Logger:           log,
}
```

**优先级逻辑:** OTel span > 请求头 > 自动生成 UUID

### 1.5 GORM Store 拆分到 `gorm/` 子包

以下包的 GORM 实现从主包拆分到独立的 `gorm/` 子包，接口定义保留在主包：

| 包 | v1 导入路径 | v2 导入路径 |
|---|---|---|
| RBAC Store | `auth/rbac` (`NewGORMStore`) | `auth/rbac/gorm` (`NewGORMStore`) |
| Outbox Store | `domain/outbox` (`NewGORMStore`) | `domain/outbox/gorm` (`NewStore` / `NewStoreFromDB`) |
| EventStore | `domain/eventsourcing` (`NewGORMEventStore`) | `domain/eventsourcing/gorm` (`NewEventStore`) |
| SnapshotStore | `domain/eventsourcing` (`NewGORMSnapshotStore`) | `domain/eventsourcing/gorm` (`NewSnapshotStore`) |
| Saga Store | `domain/saga` (`NewKVStore`) | `domain/saga/kvstore` (`NewStore`) |
| Audit Store | `bizx/audit` | `bizx/audit/gorm` |
| Retry Store | `bizx/retry` | `bizx/retry/gorm` |
| API Key Store | `llm/serving/apikey` | `llm/serving/apikey/gorm` |
| Billing Store | `llm/serving/billing` | `llm/serving/billing/gorm` |

**v1 (旧)**
```go
import "github.com/Tsukikage7/servex/auth/rbac"

store := rbac.NewGORMStore(db)
```

**v2 (新)**
```go
import (
    "github.com/Tsukikage7/servex/auth/rbac"
    rbacgorm "github.com/Tsukikage7/servex/auth/rbac/gorm"
)

store := rbacgorm.NewGORMStore(db)  // 返回 rbac.Store 接口
```

**Outbox 示例:**
```go
// v1
import "github.com/Tsukikage7/servex/domain/outbox"
store := outbox.NewGORMStore(db)

// v2
import outboxgorm "github.com/Tsukikage7/servex/domain/outbox/gorm"
store := outboxgorm.NewStore(rdbmsDB)     // 从 rdbms.Database 创建
store := outboxgorm.NewStoreFromDB(gormDB) // 从 *gorm.DB 创建
```

### 1.6 `Validatable` 接口统一到 `validation` 包

v1 中多个包各自定义了 `Validatable` 接口。v2 统一到 `validation.Validatable`，原位置改为 type alias。

```go
// v2 统一定义
import "github.com/Tsukikage7/servex/validation"

type Validatable = validation.Validatable
```

### 1.7 `httpx/activity` 解耦

- 移除对 `auth` 和 `messaging` 的直接依赖
- Kafka producer 移到 `httpx/activity/kafka/` 子包

**v1 (旧)**
```go
import "github.com/Tsukikage7/servex/httpx/activity"

producer := activity.NewKafkaProducer(...)
```

**v2 (新)**
```go
import "github.com/Tsukikage7/servex/httpx/activity/kafka"

producer := kafka.NewProducer(...)
```

### 1.8 gRPC 错误处理改用标准 `errdetails`

v1 通过 JSON-in-message 传递错误详情。v2 改用 `google.golang.org/genproto/googleapis/rpc/errdetails` 标准方式。

**v1 (旧)**
```go
// gRPC 错误内嵌 JSON
st := status.New(codes.InvalidArgument, `{"code":"INVALID","fields":[...]}`)
```

**v2 (新)**
```go
import "github.com/Tsukikage7/servex/errors"

// 自动使用 errdetails (ErrorInfo/BadRequest/RetryInfo)
st := errors.ToGRPCStatus(err)

// 从 gRPC status 提取结构化错误
appErr := errors.FromGRPCStatus(st)
```

> 向后兼容：`FromGRPCStatus` 仍能解析旧的 JSON-in-message 格式。

---

## 2. 逐步迁移

### 步骤 1: 更新 Go 版本

v2 要求 **Go 1.26+**（使用了 `iter.Seq`、`synctest` 等新特性）。

```bash
go install golang.org/dl/go1.26.1@latest
```

### 步骤 2: 更新 import 路径

使用全局替换处理 store 拆分：

```bash
# RBAC Store
sed -i 's|"github.com/Tsukikage7/servex/auth/rbac"|rbacgorm "github.com/Tsukikage7/servex/auth/rbac/gorm"|g' **/*.go

# Outbox Store
sed -i 's|outbox\.NewGORMStore|outboxgorm.NewStore|g' **/*.go

# EventSourcing Store
sed -i 's|eventsourcing\.NewGORMEventStore|esgorm.NewEventStore|g' **/*.go
```

> 建议逐文件检查而非盲目全局替换。

### 步骤 3: 迁移 Logger 调用

```bash
# 查找所有 WithContext 调用
grep -rn "\.WithContext(" --include="*.go"

# 替换为 FromContext 模式
# log.WithContext(ctx).Info(...)  →  logger.FromContext(ctx).Info(...)
```

### 步骤 4: 迁移 Server 中间件

1. 移除服务器构造中已删除的 Option 调用
2. 添加 `WithMiddlewares()` 或 `WithUnaryInterceptor()`/`WithStreamInterceptor()`
3. 显式导入中间件包（`recovery`、`logging`、`trace` 等）

### 步骤 5: 移除 `requestid` 引用

```bash
grep -rn "requestid" --include="*.go"
# 将所有引用替换为 trace 包
```

### 步骤 6: 更新 gRPC 错误处理

如果使用了自定义 gRPC 错误处理，迁移到 `errors.ToGRPCStatus`/`errors.FromGRPCStatus`。

### 步骤 7: 编译验证

```bash
go build ./...
go test ./...
```

### 步骤 8: 更新 CLI 工具

```bash
go install github.com/Tsukikage7/servex/cmd/servex@v2
```

新版 CLI 使用 `buf` 替代 `protoc`，生成的项目模板已适配 v2 架构。

---

## 3. v2 新特性

### 3.1 JWT 非对称签名

支持 RS256、ES256、EdDSA 签名方法，向后兼容 HMAC。

```go
import "github.com/Tsukikage7/servex/auth/jwt"

// RSA
j, _ := jwt.New(jwt.WithRSAKeys("private.pem", "public.pem"))

// ECDSA
j, _ := jwt.New(jwt.WithECDSAKeys("ec-private.pem", "ec-public.pem"))

// Ed25519
j, _ := jwt.New(jwt.WithEdDSAKeys("ed-private.pem", "ed-public.pem"))
```

### 3.2 OTel Metrics 导出

新增 `OTelCollector`，支持 OTLP/Prometheus/stdout 导出。

```go
import "github.com/Tsukikage7/servex/observability/metrics"

collector, _ := metrics.NewOTelCollector(metrics.OTelConfig{
    Exporter: "otlp",
    Endpoint: "localhost:4317",
})
```

### 3.3 slog 适配器

`Logger` 与标准库 `log/slog` 双向互转。

```go
import "github.com/Tsukikage7/servex/observability/logger"

// servex Logger → slog.Logger
slogLogger := logger.AsSlog(myLogger)

// slog.Logger → servex Logger
servexLogger := logger.NewFromSlog(slogLogger)

// servex Logger → slog.Handler
handler := logger.NewSlogHandler(myLogger)
```

### 3.4 iter.Seq 迭代器

所有 collections 子包支持 `All()`/`Backward()` 迭代器方法。

```go
import "github.com/Tsukikage7/servex/collections/linkedmap"

m := linkedmap.New[string, int]()
for k, v := range m.All() {
    fmt.Println(k, v)
}
```

### 3.5 `servex dev` 热重载

```bash
servex dev                          # 自动检测 main.go
servex dev --entry cmd/server/main.go
servex dev --watch ./internal --exclude vendor
```

基于 `fsnotify` 监听 `.go` 文件变更，500ms 防抖自动重启。

### 3.6 配置错误增强

`config.Load` 返回结构化错误，包含字段路径、来源和期望类型。

```go
err := config.Load(&cfg)
var fieldErr *config.ConfigFieldError
if errors.As(err, &fieldErr) {
    fmt.Println(fieldErr.Path)     // "server.http.addr"
    fmt.Println(fieldErr.Source)   // "config.yaml"
    fmt.Println(fieldErr.Expected) // "string"
    fmt.Println(fieldErr.Actual)   // "int(8080)"
}
```

### 3.7 trace 中间件增强

- 优先复用 OTel span 的 traceId/spanId，避免覆盖
- 下游传播辅助函数：`InjectHTTPHeaders`、`InjectGRPCMetadata`
- 与 `observability/tracing`（完整 OTel）可共存

### 3.8 Server 自动注入 Logger

`httpserver` 和 `grpcserver` 在最外层自动将 logger 注入到 context，所有中间件和业务代码都可通过 `logger.FromContext(ctx)` 获取。

---

## 4. 配置变更

### 4.1 Logger 默认输出格式

| 项目 | v1 | v2 |
|------|----|----|
| 格式 | JSON | Console |
| Caller | 关闭 | 开启 (short) |
| 字段样式 | JSON 结构体 | `[key:value]` 同行显示 |

如需保持 v1 行为：

```yaml
logger:
  format: json
  enableCaller: false
```

### 4.2 GRPCConfig 移除 `PublicMethods` 字段

v1 的 `transport.GRPCConfig` 包含 `PublicMethods []string` 字段，v2 已移除。
认证公开方法配置改为在 auth 中间件中单独设置。

### 4.3 Server 构造函数

`httpserver.New` 和 `grpcserver.New` 现在强制要求 `WithLogger()`，未设置时 panic。

---

## 5. 废弃项

以下在 v2 中已标记废弃或计划在后续版本移除：

| 项目 | 状态 | 替代方案 |
|------|------|---------|
| `middleware/requestid` | **已删除** | `middleware/trace` |
| `Logger.WithContext()` | **已删除** | `logger.NewContext()` + `logger.FromContext()` |
| Server 内置中间件 Option | **已删除** | `WithMiddlewares()` 显式组合 |
| gRPC JSON-in-message 错误 | 废弃 (可读) | `errors.ToGRPCStatus()` (errdetails) |
| 主包内 GORM Store 构造函数 | **已删除** | `xxx/gorm` 子包 |
| `httpx/activity` Kafka producer | **已移动** | `httpx/activity/kafka` 子包 |

---

## 快速检查清单

- [ ] Go >= 1.26
- [ ] 所有 `log.WithContext(ctx)` 替换为 `logger.FromContext(ctx)`
- [ ] Server 构造中删除已移除的内置中间件 Option
- [ ] 添加 `WithMiddlewares()` 显式组合所需中间件
- [ ] GORM Store 导入路径更新到 `gorm/` 子包
- [ ] `middleware/requestid` 引用替换为 `middleware/trace`
- [ ] gRPC 错误处理迁移到 `errors.ToGRPCStatus()`
- [ ] `go build ./...` 编译通过
- [ ] `go test ./...` 测试通过
