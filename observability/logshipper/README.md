# observability/logshipper

异步日志投递模块，将结构化日志批量投递到 Elasticsearch 或 Kafka 等外部存储，支持与 `zap.Logger` 和 `logger.Logger` 接口无缝集成。

## 核心概念

- **Entry** — 结构化日志条目（timestamp/level/message/fields）
- **Sink** — 投递目标接口，内置 ES sink 和 Kafka sink，可自定义实现
- **Shipper** — 投递器，维护缓冲 channel，后台协程批量写入 Sink

## 快速开始

```go
ctx := context.Background()

// 1. 创建 Sink（ES 或 Kafka，见下方）
sink := logshipper.NewElasticsearchSink(esClient)

// 2. 创建并启动 Shipper
s := logshipper.New(sink,
    logshipper.WithBatchSize(200),
    logshipper.WithFlushInterval(3*time.Second),
    logshipper.WithBufferSize(20000),
    logshipper.WithDropOnFull(true),
    logshipper.WithErrorHandler(func(err error) { log.Println("ship error:", err) }),
)
s.Start(ctx)
defer s.Close()

// 3. 手动投递（通常通过 Hook 自动触发）
s.Ship(logshipper.Entry{
    Timestamp: time.Now(),
    Level:     "info",
    Message:   "服务启动",
    Fields:    map[string]any{"version": "1.0.0"},
})
```

## Elasticsearch Sink

```go
import "github.com/Tsukikage7/servex/v2/storage/elasticsearch"

esClient, _ := elasticsearch.NewClient(esCfg, log)
sink := logshipper.NewElasticsearchSink(esClient,
    logshipper.WithIndexPrefix("app-logs-"), // 默认 "logs-"
    logshipper.WithDateSuffix("2006.01.02"), // 默认按日分索引
)
// 写入索引示例：app-logs-2026.04.05
```

## Kafka Sink

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"

publisher, _ := kafka.NewPublisher(kafkaCfg)
sink := logshipper.NewKafkaSink(publisher,
    logshipper.WithTopic("app-logs"), // 默认 "logs"
)
```

## Hook 集成

### 方式一：ZapHook（直接持有 *zap.Logger）

```go
hook := logshipper.ZapHook(shipper)
zapLogger = zap.New(zapcore.NewTee(originalCore, hook))
```

### 方式二：AttachToLogger（封装已有 *zap.Logger）

```go
zapLogger = logshipper.AttachToLogger(zapLogger, shipper)
```

### 方式三：NewLoggerHook（适配 logger.Logger 接口）

```go
hooked := logshipper.NewLoggerHook(innerLogger, shipper, "info")
// debug 日志不投递，info 及以上才投递
hooked.Infof("请求完成: %v", requestID)
```

## Shipper 选项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithBatchSize(n)` | 100 | 达到 n 条立即 flush |
| `WithFlushInterval(d)` | 5s | 定时 flush 间隔 |
| `WithBufferSize(n)` | 10000 | 缓冲 channel 大小 |
| `WithDropOnFull(bool)` | true | 缓冲满时丢弃（false 则阻塞） |
| `WithErrorHandler(fn)` | nop | 投递失败回调 |

## 生命周期

`s.Start(ctx)` 启动后台协程；`ctx` 取消或调用 `s.Close()` 时均会排空缓冲区后退出，`Close()` 幂等可安全多次调用。
