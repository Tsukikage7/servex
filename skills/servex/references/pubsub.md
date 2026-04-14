# servex 消息与任务

## Config 驱动用法（推荐）

### Pub/Sub Factory

```go
import (
    "github.com/Tsukikage7/servex/v2/messaging/pubsub"
    "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
)

// factory.Config 支持 "kafka", "rabbitmq", "redis" 三种 Type
pub, _ := factory.NewPublisher(&factory.Config{
    Type:    "kafka",
    Brokers: []string{"localhost:9092"},
}, log)
defer pub.Close()

pub.Publish(ctx, "orders", &pubsub.Message{
    Key:  []byte("order-123"),
    Body: data,
})

// NewSubscriber 额外接受 group 参数（用于 Kafka/Redis consumer group）
sub, _ := factory.NewSubscriber(&factory.Config{
    Type:    "kafka",
    Brokers: []string{"localhost:9092"},
}, "my-group", log)
defer sub.Close()

ch, _ := sub.Subscribe(ctx, "orders")
for msg := range ch {
    process(msg)
    sub.Ack(ctx, msg)
}
```

### JobQueue Factory

```go
import (
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
)

// factory.StoreConfig 支持 "redis", "kafka", "rabbitmq", "database" 四种 Type
store, _ := factory.NewStore(&factory.StoreConfig{
    Type: "redis",
    Addr: "localhost:6379",
})

client := jobqueue.NewClient(store)
client.Enqueue(ctx, &jobqueue.Job{
    Queue: "emails", Type: "welcome",
    Payload: payload, Priority: 3, MaxRetries: 5,
    Delay: 10 * time.Minute,
})

w := jobqueue.NewWorker(store,
    jobqueue.WithQueues("emails", "reports"),
    jobqueue.WithConcurrency(10),
)
w.Register("welcome", sendWelcomeEmail)
w.Start(ctx)
```

## Pub/Sub 统一接口（driver 级别）

### 核心接口

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub"

// Message 是传输的基本单元
type Message struct {
    ID       string
    Topic    string
    Key      []byte
    Body     []byte
    Headers  map[string]string
    Metadata map[string]any // driver 特有信息
}

type Publisher interface {
    Publish(ctx context.Context, topic string, msgs ...*Message) error
    Close() error
}

type Subscriber interface {
    Subscribe(ctx context.Context, topic string) (<-chan *Message, error)
    Ack(ctx context.Context, msg *Message) error
    Nack(ctx context.Context, msg *Message) error
    Close() error
}
```

### Kafka

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"

// 构造函数接收已有的 sarama.Client
pub, _ := kafka.NewPublisher(saramaClient,
    kafka.WithPublisherLogger(log),
)
defer pub.Close()

pub.Publish(ctx, "orders", &pubsub.Message{
    Key:  []byte("order-123"),
    Body: data,
    Headers: map[string]string{"source": "api"},
})

sub, _ := kafka.NewSubscriber(saramaClient, "my-group",
    kafka.WithSubscriberLogger(log),
)
defer sub.Close()

ch, _ := sub.Subscribe(ctx, "orders")
for msg := range ch {
    process(msg)
    sub.Ack(ctx, msg) // 显式确认
}
```

### RabbitMQ

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"

// 构造函数接收已有的 *amqp.Connection
pub, _ := rabbitmq.NewPublisher(amqpConn,
    rabbitmq.WithExchange("events", "topic"),
    rabbitmq.WithDurable(true),
    rabbitmq.WithConfirm(true),
)

sub, _ := rabbitmq.NewSubscriber(amqpConn,
    rabbitmq.WithSubscriberExchange("events", "topic"),
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithAutoAck(false),
)
```

### Redis Streams

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"

// 构造函数接收已有的 *redis.Client
pub, _ := redis.NewPublisher(redisClient,
    redis.WithMaxLen(10000),
)

sub, _ := redis.NewSubscriber(redisClient,
    redis.WithGroup("my-group"),
    redis.WithConsumer("worker-1"),
    redis.WithBlockTime(5 * time.Second),
)
```

## JobQueue 异步任务队列

### 核心接口

```go
import "github.com/Tsukikage7/servex/v2/messaging/jobqueue"

// Job 表示一个异步任务
type Job struct {
    ID, Queue, Type string
    Payload         []byte
    Priority        int           // 数值越大优先级越高
    MaxRetries      int
    Delay           time.Duration // 延迟执行
    Deadline        time.Time
}

type Handler func(ctx context.Context, job *Job) error

type Client interface { Enqueue(ctx, job) error; Close() error }
type Worker interface { Register(jobType, handler); Start(ctx) error; Close() error }
type Store  interface { Enqueue/Dequeue/MarkRunning/MarkFailed/MarkDead/MarkDone/Requeue/ListDead/Close }
```

### 投递与消费

```go
import (
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    jqredis "github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
)

store, _ := jqredis.NewStore(redisClient, jqredis.WithPrefix("myapp"))

// 投递端
client := jobqueue.NewClient(store)
client.Enqueue(ctx, &jobqueue.Job{
    Queue: "emails", Type: "welcome",
    Payload: payload, Priority: 3, MaxRetries: 5,
    Delay: 10 * time.Minute,
})

// 消费端
w := jobqueue.NewWorker(store,
    jobqueue.WithQueues("emails", "reports"),
    jobqueue.WithConcurrency(10),
    jobqueue.WithPollInterval(time.Second),
)
w.Register("welcome", sendWelcomeEmail)
w.Register("report", generateReport)
w.Start(ctx) // 阻塞，ctx 取消后优雅退出
```

### Store 后端

| Store | 包路径 | 构造函数参数 | 特点 |
|-------|--------|-------------|------|
| Redis | `jobqueue/redis` | `*redis.Client` | sorted set 延迟队列，高吞吐 |
| Kafka | `jobqueue/kafka` | `sarama.Client` | topic 作为队列，极高吞吐 |
| RabbitMQ | `jobqueue/rabbitmq` | `*amqp.Connection` | 原生 dead letter exchange |
| Database | `jobqueue/database` | `*gorm.DB` | 无额外依赖，乐观锁 dequeue |

## eventbus — 进程内事件总线

**包路径：** `github.com/Tsukikage7/servex/messaging/eventbus`

**何时使用：** 进程内同步/异步事件分发，基于主题的发布订阅。区别于 `bizx/event`（通配符/优先级），eventbus 更轻量，基于 Event 接口的强类型事件。

### 核心接口

```go
// 所有事件必须实现 Event 接口
type Event interface {
    Topic() string
}

// 事件处理函数
type Handler func(ctx context.Context, event Event) error
```

### 构造函数

- `New(opts...)` — 创建事件总线
- `WithAsyncWorkers(n)` — 异步工作协程数量（默认 4）
- `WithLogger(l)` — 设置日志记录器
- `WithErrorHandler(fn)` — 异步错误处理回调

### 示例

```go
import "github.com/Tsukikage7/servex/v2/messaging/eventbus"

// 定义事件
type OrderCreated struct {
    OrderID string
}
func (e OrderCreated) Topic() string { return "order.created" }

// 创建事件总线
bus := eventbus.New(
    eventbus.WithAsyncWorkers(8),
    eventbus.WithErrorHandler(func(err error) {
        log.Printf("事件处理失败: %v", err)
    }),
)
defer bus.Close()

// 订阅事件（返回取消订阅函数）
unsub := bus.Subscribe("order.created", func(ctx context.Context, event eventbus.Event) error {
    e := event.(OrderCreated)
    fmt.Printf("订单创建: %s\n", e.OrderID)
    return nil
})
defer unsub()

// 订阅所有事件（通配符）
unsubAll := bus.SubscribeAll(func(ctx context.Context, event eventbus.Event) error {
    fmt.Printf("收到事件: %s\n", event.Topic())
    return nil
})

// 同步发布（按顺序调用所有处理器，任一失败立即返回错误）
err := bus.Publish(ctx, OrderCreated{OrderID: "123"})

// 异步发布（分发给工作协程处理）
bus.PublishAsync(ctx, OrderCreated{OrderID: "456"})
```

**关键类型：**
- `eventbus.Bus` — 事件总线（`Subscribe`, `SubscribeAll`, `Publish`, `PublishAsync`, `Close`）
- `eventbus.Event` — 事件接口（需实现 `Topic() string`）
- `eventbus.Handler` — 处理函数 `func(ctx, Event) error`
- 错误：`ErrBusClosed` — 总线已关闭
