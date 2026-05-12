# jobqueue

`github.com/Tsukikage7/servex/jobqueue` -- 异步任务队列。

## 概述

jobqueue 包提供轻量级的异步任务队列框架，支持延迟任务、优先级、自动重试和死信队列。业务代码面向 Client/Worker/Store 三层接口编程，通过切换 Store 后端即可迁移到不同存储系统。

## 功能特性

- 异步投递：Client 投递任务，Worker 异步消费
- 延迟任务：通过 Job.Delay 设置延迟执行时间
- 优先级队列：高优先级任务优先执行
- 自动重试：失败任务按指数退避重试，超过 MaxRetries 进入死信队列
- 多种后端：Database（GORM）、Redis、Kafka、RabbitMQ 四种 Store 实现
- 并发控制：Worker 支持配置并发数

## 任务状态流转

```
pending --Dequeue--> running --成功--> done（删除）
                       |
                       +--失败(retried < maxRetries)--> pending（重新入队）
                       |
                       +--失败(retried >= maxRetries)--> dead
```

## 核心接口

### Job

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `string` | 任务 ID（自动生成 UUID） |
| `Queue` | `string` | 队列名称 |
| `Type` | `string` | 任务类型，用于匹配 Handler |
| `Payload` | `[]byte` | 任务载荷 |
| `Priority` | `int` | 优先级（越大越优先） |
| `MaxRetries` | `int` | 最大重试次数 |
| `Retried` | `int` | 已重试次数 |
| `Delay` | `time.Duration` | 延迟执行时间 |
| `Deadline` | `time.Time` | 截止时间 |
| `Status` | `Status` | 任务状态 |
| `LastError` | `string` | 最后一次错误信息 |

### Status 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `StatusPending` | `"pending"` | 待执行 |
| `StatusRunning` | `"running"` | 执行中 |
| `StatusFailed` | `"failed"` | 执行失败 |
| `StatusDead` | `"dead"` | 死信（重试耗尽） |

### 构造函数

| 函数 | 说明 |
|------|------|
| `NewClient(store Store) Client` | 创建任务投递客户端 |
| `NewWorker(store Store, opts ...WorkerOption) Worker` | 创建任务消费 Worker |

### Client 接口

| 方法 | 说明 |
|------|------|
| `Enqueue(ctx, job) error` | 投递任务 |
| `Close() error` | 关闭客户端 |

### Worker 接口

| 方法 | 说明 |
|------|------|
| `Register(jobType string, handler Handler)` | 注册任务处理函数 |
| `Start(ctx) error` | 启动 Worker，阻塞直到 ctx 取消 |
| `Close() error` | 关闭 Worker |

### Store 接口

| 方法 | 说明 |
|------|------|
| `Enqueue(ctx, job) error` | 入队 |
| `Dequeue(ctx, queue) (*Job, error)` | 出队 |
| `MarkRunning(ctx, id) error` | 标记执行中 |
| `MarkFailed(ctx, id, err) error` | 标记失败 |
| `MarkDead(ctx, id) error` | 标记死信 |
| `MarkDone(ctx, id) error` | 标记完成 |
| `Requeue(ctx, job) error` | 重新入队 |
| `ListDead(ctx, queue) ([]*Job, error)` | 列出死信任务 |
| `Close() error` | 关闭 |

### Worker 选项 (WorkerOption)

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithQueues(queues ...string)` | - | 要消费的队列名称（必填） |
| `WithConcurrency(n int)` | `1` | 并发处理数 |
| `WithPollInterval(d time.Duration)` | `1s` | 轮询间隔 |
| `WithLogger(logger.Logger)` | - | 设置日志记录器 |

## Store 后端对比

| 后端 | 包路径 | 构造函数 | 特点 |
|------|--------|----------|------|
| Database | `jobqueue/database` | `NewStore(db *gorm.DB, opts ...Option)` | GORM 实现，乐观锁出队，支持优先级排序 |
| Redis | `jobqueue/redis` | `NewStore(client *goredis.Client, opts ...Option)` | Sorted Set 实现，天然支持延迟和优先级 |
| Kafka | `jobqueue/kafka` | `NewStore(client sarama.Client, opts ...Option)` | 仅支持 Enqueue（投递），Dequeue 需 consumer group |
| RabbitMQ | `jobqueue/rabbitmq` | `NewStore(conn *amqp.Connection, opts ...Option)` | 支持死信队列路由，消息持久化 |

各后端通用选项：`WithPrefix(string)` 设置键/表/topic 前缀（Database 使用 `WithTableName`）。

## 使用示例

### Config 驱动（按需注册）

`jobqueue/factory` 核心包只维护配置和注册表，不直接导入 Redis/Kafka/RabbitMQ/Database 后端，避免拉入当前不用的间接依赖。使用哪个后端，就 blank import 对应注册包。

```go
import (
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
    _ "github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory/redis"
)

// Redis Store
store, _ := factory.NewStore(&factory.StoreConfig{
    Type: "redis",
    Addr: "localhost:6379",
})

// 投递端
client := jobqueue.NewClient(store)
defer client.Close()

client.Enqueue(ctx, &jobqueue.Job{
    Queue:      "emails",
    Type:       "send_welcome",
    Payload:    []byte(`{"user_id":"123"}`),
    MaxRetries: 3,
    Delay:      5 * time.Second,
})

// 消费端
w := jobqueue.NewWorker(store,
    jobqueue.WithQueues("emails", "notifications"),
    jobqueue.WithConcurrency(5),
    jobqueue.WithPollInterval(500*time.Millisecond),
)

w.Register("send_welcome", func(ctx context.Context, job *jobqueue.Job) error {
    var payload map[string]string
    json.Unmarshal(job.Payload, &payload)
    return sendEmail(ctx, payload["user_id"])
})

// 阻塞运行，直到 ctx 取消
w.Start(ctx)
```

`StoreConfig` 结构体：

| 字段 | 类型 | 适用后端 | 说明 |
|------|------|----------|------|
| `Type` | `string` | 全部 | `"redis"`, `"kafka"`, `"rabbitmq"`, `"database"` |
| `Addr` | `string` | Redis | Redis 地址 |
| `Password` | `string` | Redis | Redis 密码 |
| `DB` | `int` | Redis | Redis DB 编号 |
| `Prefix` | `string` | Redis/Kafka | 键/topic 前缀 |
| `Brokers` | `[]string` | Kafka | Kafka broker 地址列表 |
| `URL` | `string` | RabbitMQ | AMQP URL |
| `Driver` | `string` | Database | `"mysql"`, `"postgres"`, `"sqlite"` |
| `DSN` | `string` | Database | 数据库连接字符串 |
| `Table` | `string` | Database | 表名 |

按需注册包：

| Type | 注册包 |
|------|--------|
| `redis` | `messaging/jobqueue/factory/redis` |
| `kafka` | `messaging/jobqueue/factory/kafka` |
| `rabbitmq` | `messaging/jobqueue/factory/rabbitmq` |
| `database` | `messaging/jobqueue/factory/database` |

### 高级用法（直接使用 driver）

```go
import (
    "github.com/Tsukikage7/servex/jobqueue"
    "github.com/Tsukikage7/servex/jobqueue/redis"
)

// 创建 Store（复用已有 *redis.Client）
store, _ := redis.NewStore(redisClient, redis.WithPrefix("myapp"))

// --- 投递端 ---
client := jobqueue.NewClient(store)
defer client.Close()

client.Enqueue(ctx, &jobqueue.Job{
    Queue:      "emails",
    Type:       "send_welcome",
    Payload:    []byte(`{"user_id":"123"}`),
    MaxRetries: 3,
    Delay:      5 * time.Second,
})

// --- 消费端 ---
w := jobqueue.NewWorker(store,
    jobqueue.WithQueues("emails", "notifications"),
    jobqueue.WithConcurrency(5),
    jobqueue.WithPollInterval(500*time.Millisecond),
)

w.Register("send_welcome", func(ctx context.Context, job *jobqueue.Job) error {
    var payload map[string]string
    json.Unmarshal(job.Payload, &payload)
    return sendEmail(ctx, payload["user_id"])
})

// 阻塞运行，直到 ctx 取消
w.Start(ctx)
```
