# Logger

高性能结构化日志库，基于 [uber-go/zap](https://github.com/uber-go/zap) 构建，为微服务提供统一的日志解决方案。

## 特性

- 高性能：基于 zap 的零分配日志记录
- 多输出：支持控制台、文件、同时输出
- 日志轮转：支持按天/小时自动轮转
- 级别分离：可按日志级别输出到不同文件
- 结构化：支持添加结构化字段
- 上下文：支持通过 context 注入 logger，并在 fallback 场景自动附加 traceId/spanId
- 预设配置：提供开发、生产环境预设配置

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/observability/logger
```

## 配置选项

### 完整配置示例

```go
config := &logger.Config{
    // 基础配置
    Type:        logger.TypeZap,      // 日志实现类型
    Level:       logger.LevelInfo,    // 日志级别: debug, info, warn, error, fatal, panic
    Format:      logger.FormatJSON,   // 输出格式: json, console
    Output:      logger.OutputBoth,   // 输出目标: console, file, both

    // 文件输出配置
    LogDir:      "/var/log/app",      // 日志目录
    ServiceName: "my-service",        // 服务名（用于文件名前缀）

    // 轮转配置
    RotationEnabled: true,                    // 启用日志轮转
    RotationTime:    logger.RotationDaily,    // 轮转周期: daily, hourly
    MaxAge:          30,                      // 日志保留天数
    Compress:        true,                    // 压缩旧日志

    // 高级配置
    LevelSeparate:      false,  // 按级别分离文件
    ConsoleEnabled:     true,   // 文件输出时同时输出到控制台
    EnableCaller:       true,   // 记录调用位置
    CallerSkip:         1,      // 调用栈跳过层数
    EnableStacktrace:   true,   // Error 级别记录堆栈

    // 编码配置
    TimeKey:      "timestamp",                  // 时间字段名
    LevelKey:     "level",                      // 级别字段名
    MessageKey:   "msg",                        // 消息字段名
    CallerKey:    "caller",                     // 调用位置字段名
    TimeFormat:   logger.TimeFormatDateTime,   // 时间格式
    EncodeLevel:  logger.EncodeLevelCapital,   // 级别编码: capital, capitalColor, lower, lowerColor
    EncodeCaller: logger.EncodeCallerShort,    // 调用位置编码: short, full
}

log, err := logger.NewLogger(config)
```

### 配置说明

| 配置项            | 类型   | 默认值    | 说明                       |
| ----------------- | ------ | --------- | -------------------------- |
| `Level`           | string | `info`    | 日志级别                   |
| `Format`          | string | `console` | 输出格式                   |
| `Output`          | string | `console` | 输出目标                   |
| `LogDir`          | string | -         | 日志目录（文件输出时必填） |
| `ServiceName`     | string | `service` | 服务名                     |
| `RotationEnabled` | bool   | `false`   | 启用轮转                   |
| `RotationTime`    | string | `daily`   | 轮转周期                   |
| `MaxAge`          | int    | `7`       | 保留天数                   |
| `Compress`        | bool   | `false`   | 压缩旧日志                 |
| `LevelSeparate`   | bool   | `false`   | 按级别分离文件             |

## 结构化日志

### 添加字段

```go
// 推荐：消息正文 + 结构化字段
log.Info("[User] 执行动作",
    logger.String("user_id", "12345"),
    logger.Int("age", 25),
    logger.Bool("vip", true),
)
// 输出: {"level":"INFO","timestamp":"2024-01-15 10:30:45","msg":"[User] 执行动作","user_id":"12345","age":25,"vip":true}

// 需要绑定固定字段时使用 With
authLog := log.With(logger.Component("Auth"))
authLog.Warn("主体已过期", logger.String("principal_id", "u_123"))
```

### 支持的字段类型

```go
logger.String("key", "value")           // 字符串
logger.Int("key", 42)                   // 整数
logger.Int64("key", 9223372036854775807) // int64
logger.Float64("key", 3.14)             // 浮点数
logger.Bool("key", true)                // 布尔值
logger.Time("key", time.Now())          // 时间
logger.Duration("key", 5*time.Second)   // 持续时间
logger.Err(err)                         // 错误（固定 key 为 "error"）
logger.Any("key", anyValue)             // 任意类型
```

## 上下文集成

### 请求上下文使用（推荐）

HTTP server、gRPC server 和 Gateway 会把 logger 注入请求 context。业务代码和中间件优先从 context 读取：

```go
import (
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

func handleRequest(w http.ResponseWriter, r *http.Request) {
    logger.FromContext(r.Context()).Info("[HTTP] 处理请求",
        logger.String("method", r.Method),
        logger.String("path", r.URL.Path),
    )
}
```

### 独立中间件 fallback

独立使用中间件时，context 中可能还没有 logger。此时使用 `FromContextOr`，优先读取 context，缺失时回退到传入的 logger：

```go
func Middleware(log logger.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            logger.FromContextOr(r.Context(), log).Info("[HTTP] 请求完成",
                logger.String("path", r.URL.Path),
            )
            next.ServeHTTP(w, r)
        })
    }
}
```

### 手动注入 context

```go
ctx := context.Background()
log := logger.MustNewLogger(logger.DefaultConfig())
ctx = logger.NewContext(ctx, log.With(logger.String("request_id", "req_123")))

logger.FromContext(ctx).Info("[Worker] 处理任务")
```

### 在 HTTP 中间件中使用

结合 trace 包使用（推荐）：

```go
import (
    "github.com/Tsukikage7/servex/v2/observability/logger"
    "github.com/Tsukikage7/servex/v2/middleware/trace"
)

func main() {
    log, _ := logger.NewLogger(logger.DefaultConfig())

    // 使用 trace 中间件（自动生成 traceId 并注入 logger 到 context）
    mux := http.NewServeMux()
    handler := trace.HTTPMiddleware(&trace.Config{Logger: log})(mux)

    http.ListenAndServe(":8080", handler)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 自动携带 traceId 和 spanId
    logger.FromContext(ctx).Info("[HTTP] 请求开始",
        logger.String("method", r.Method),
        logger.String("path", r.URL.Path),
    )
}
```

## 日志级别分离

当需要将不同级别的日志写入不同文件时：

```go
config := &logger.Config{
    Level:           logger.LevelDebug,
    Output:          logger.OutputFile,
    LogDir:          "/var/log/app",
    LevelSeparate:   true,  // 启用级别分离
    RotationEnabled: true,
}

log, _ := logger.NewLogger(config)
defer log.Close()

log.Debug("debug message")  // 写入 /var/log/app/debug/debug_2024-01-15.log
log.Info("info message")    // 写入 /var/log/app/info/info_2024-01-15.log
log.Error("error message")  // 写入 /var/log/app/error/error_2024-01-15.log
```

## 日志轮转

### 按天轮转

```go
config := &logger.Config{
    Output:          logger.OutputFile,
    LogDir:          "/var/log/app",
    ServiceName:     "my-service",
    RotationEnabled: true,
    RotationTime:    logger.RotationDaily,
    MaxAge:          30,      // 保留 30 天
    Compress:        true,    // 压缩旧日志
}
// 生成文件: /var/log/app/my-service/my-service_2024-01-15.log
```

### 按小时轮转

```go
config := &logger.Config{
    Output:          logger.OutputFile,
    LogDir:          "/var/log/app",
    ServiceName:     "my-service",
    RotationEnabled: true,
    RotationTime:    logger.RotationHourly,
    MaxAge:          7,
}
// 生成文件: /var/log/app/my-service/my-service_2024-01-15_10.log
```

## 最佳实践

### 1. 应用启动时初始化全局 logger

```go
var log logger.Logger

func init() {
    var err error
    if os.Getenv("ENV") == "production" {
        log, err = logger.NewLogger(logger.NewProdConfig(
            os.Getenv("SERVICE_NAME"),
            os.Getenv("LOG_DIR"),
        ))
    } else {
        log, err = logger.NewLogger(logger.NewDevConfig())
    }
    if err != nil {
        panic(err)
    }
}

func main() {
    defer log.Close()
    // ...
}
```

### 2. 在请求处理中使用 logger

```go
func HandleRequest(ctx context.Context) {
    // 从 context 取出 logger（已由 trace 中间件注入 traceId）
    log := logger.FromContext(ctx)

    log.Info("[Handler] 处理请求",
        logger.String("handler", "HandleRequest"),
    )
    // 业务逻辑...
}
```

### 3. 错误日志包含堆栈信息

```go
config := logger.NewProdConfig("my-service", "/var/log/app")
config.EnableStacktrace = true  // Error 级别自动记录堆栈

log, _ := logger.NewLogger(config)

log.Error("database connection failed",
    logger.Err(err),
    logger.String("host", "db.example.com"),
)
```

### 4. 优雅关闭

```go
func main() {
    log, _ := logger.NewLogger(logger.DefaultConfig())

    // 确保程序退出时刷新并关闭日志
    defer func() {
        if err := log.Sync(); err != nil {
            // 忽略 stdout/stderr sync 错误
        }
        log.Close()
    }()

    // 信号处理
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    <-sigCh
    log.Info("shutting down...")
}
```

## 常量参考

### 日志级别

| 常量         | 值      | 说明                   |
| ------------ | ------- | ---------------------- |
| `LevelDebug` | `debug` | 调试信息               |
| `LevelInfo`  | `info`  | 一般信息               |
| `LevelWarn`  | `warn`  | 警告信息               |
| `LevelError` | `error` | 错误信息               |
| `LevelFatal` | `fatal` | 致命错误（记录后退出） |
| `LevelPanic` | `panic` | 恐慌（记录后 panic）   |

### 输出格式

| 常量            | 值        | 说明           |
| --------------- | --------- | -------------- |
| `FormatJSON`    | `json`    | JSON 格式      |
| `FormatConsole` | `console` | 控制台友好格式 |

### 输出目标

| 常量            | 值        | 说明                   |
| --------------- | --------- | ---------------------- |
| `OutputConsole` | `console` | 输出到控制台           |
| `OutputFile`    | `file`    | 输出到文件             |
| `OutputBoth`    | `both`    | 同时输出到控制台和文件 |

### 时间格式

| 常量                    | 值            | 示例                           |
| ----------------------- | ------------- | ------------------------------ |
| `TimeFormatISO8601`     | `iso8601`     | 2024-01-15T10:30:45.000Z       |
| `TimeFormatRFC3339`     | `rfc3339`     | 2024-01-15T10:30:45Z           |
| `TimeFormatRFC3339Nano` | `rfc3339nano` | 2024-01-15T10:30:45.123456789Z |
| `TimeFormatEpoch`       | `epoch`       | 1705315845                     |
| `TimeFormatEpochMillis` | `epochMillis` | 1705315845000                  |
| `TimeFormatEpochNanos`  | `epochNanos`  | 1705315845000000000            |
| `TimeFormatDateTime`    | `datetime`    | 2024-01-15 10:30:45            |

## 测试覆盖率

```bash
go test ./logger/... -cover
# coverage: 94.3% of statements
```

## License

MIT License
