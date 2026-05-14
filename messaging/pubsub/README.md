# pubsub

`github.com/Tsukikage7/servex/v2/messaging/pubsub` -- 统一 Pub/Sub 消息抽象层。

## 概述

pubsub 包提供统一的发布/订阅接口，屏蔽底层消息中间件差异。业务代码面向 `Publisher`/`Subscriber` 接口编程，通过切换 driver 即可迁移到不同消息系统，无需修改业务逻辑。

## 功能特性

- 统一抽象：Publisher/Subscriber/Message 三层抽象，接口简洁
- 三种 driver：Kafka、RabbitMQ、Redis Streams，按需导入
- 手动确认：Ack/Nack 语义统一，各 driver 映射到原生机制
- 消息元数据：Headers 和 Metadata 透传中间件特有信息
- 幂等关闭：所有 driver 的 Close() 均可安全多次调用

## 核心接口

### Message

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `string` | 消息 ID |
| `Topic` | `string` | 主题 |
| `Key` | `[]byte` | 分区键 |
| `Body` | `[]byte` | 消息体 |
| `Headers` | `map[string]string` | 消息头 |
| `Metadata` | `map[string]any` | 中间件元数据（partition/offset 等） |

### Publisher

| 方法 | 说明 |
|------|------|
| `Publish(ctx, topic, msgs...) error` | 发布一条或多条消息到指定 topic |
| `Close() error` | 关闭 publisher |

### Subscriber

| 方法 | 说明 |
|------|------|
| `Subscribe(ctx, topic) (<-chan *Message, error)` | 订阅 topic，返回消息 channel |
| `Ack(ctx, msg) error` | 确认消息已处理 |
| `Nack(ctx, msg) error` | 拒绝消息 |
| `Close() error` | 关闭 subscriber |

## Driver

### Kafka (`pubsub/kafka`)

基于 `github.com/IBM/sarama`，使用 Consumer Group 消费。

| 构造函数 | 说明 |
|----------|------|
| `NewPublisher(client sarama.Client, opts ...PublisherOption) (*Publisher, error)` | 基于已有 sarama.Client 创建 |
| `NewSubscriber(client sarama.Client, groupID string, opts ...SubscriberOption) (*Subscriber, error)` | 基于已有 sarama.Client 创建 |

| 选项 | 说明 |
|------|------|
| `WithPublisherLogger(logger.Logger)` | 设置 Publisher 日志器 |
| `WithSubscriberLogger(logger.Logger)` | 设置 Subscriber 日志器 |

### RabbitMQ (`pubsub/rabbitmq`)

基于 `github.com/rabbitmq/amqp091-go`，支持交换机和发布确认。

| 构造函数 | 说明 |
|----------|------|
| `NewPublisher(url string, opts ...PublisherOption) (*Publisher, error)` | 基于 AMQP URL 创建 |
| `NewSubscriber(url string, opts ...SubscriberOption) (*Subscriber, error)` | 基于 AMQP URL 创建 |

**Publisher 选项**

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithExchange(name, typ)` | 无交换机 | 设置交换机名称和类型（direct/fanout/topic） |
| `WithPublisherConfirm(bool)` | `true` | 开启发布确认 |
| `WithPublisherDurable(bool)` | `true` | 交换机持久化 |
| `WithPublisherLogger(logger.Logger)` | - | 设置日志器 |

**Subscriber 选项**

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithSubscriberExchange(name, typ)` | 无交换机 | 设置交换机名称和类型 |
| `WithSubscriberDurable(bool)` | `true` | 队列持久化 |
| `WithAutoAck(bool)` | `false` | 自动确认消息 |
| `WithPrefetchCount(int)` | `10` | 预取消息数量 |
| `WithSubscriberLogger(logger.Logger)` | - | 设置日志器 |

### Redis Streams (`pubsub/redis`)

基于 `github.com/redis/go-redis/v9`，支持消费者组（XREADGROUP）和简单读取（XREAD）。

| 构造函数 | 说明 |
|----------|------|
| `NewPublisher(client goredis.Cmdable, opts ...PublisherOption) (*Publisher, error)` | 基于已有 redis 客户端创建 |
| `NewSubscriber(client goredis.Cmdable, opts ...SubscriberOption) (*Subscriber, error)` | 基于已有 redis 客户端创建 |

**Publisher 选项**

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithMaxLen(maxLen int64, approx bool)` | 无限制 | Stream 最大长度，approx=true 使用近似裁剪 |
| `WithPublisherLogger(logger.Logger)` | - | 设置日志器 |

**Subscriber 选项**

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithConsumerGroup(group, consumer)` | 无（使用 XREAD） | 设置消费者组，启用 XREADGROUP 模式 |
| `WithBlock(bool)` | `true` | 是否阻塞读取 |
| `WithSubscriberLogger(logger.Logger)` | - | 设置日志器 |

## 使用示例

### Config 驱动（按需注册）

通过 `messaging/pubsub/factory` 包，只需一个 `Config` 即可创建 Publisher/Subscriber。factory 核心包只维护配置和注册表，不直接导入 Kafka、RabbitMQ 或 Redis Streams 后端，避免拉入当前不用的间接依赖。使用哪个后端，就 blank import 对应注册包。

```go
import (
    "github.com/Tsukikage7/servex/v2/messaging/pubsub"
    "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
    _ "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory/kafka"
    _ "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory/rabbitmq"
    _ "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory/redis"
)

// Kafka
pub, _ := factory.NewPublisher(&factory.Config{
    Type:    "kafka",
    Brokers: []string{"localhost:9092"},
}, log)
defer pub.Close()

pub.Publish(ctx, "orders", &pubsub.Message{
    Key:  []byte("order-123"),
    Body: []byte(`{"id":"123"}`),
})

sub, _ := factory.NewSubscriber(&factory.Config{
    Type:    "kafka",
    Brokers: []string{"localhost:9092"},
}, "my-group", log)
defer sub.Close()

ch, _ := sub.Subscribe(ctx, "orders")
for msg := range ch {
    fmt.Println(string(msg.Body))
    sub.Ack(ctx, msg)
}

// RabbitMQ
pub, _ := factory.NewPublisher(&factory.Config{
    Type: "rabbitmq",
    URL:  "amqp://localhost",
}, log)

// Redis Streams
pub, _ := factory.NewPublisher(&factory.Config{
    Type: "redis",
    Addr: "localhost:6379",
}, log)
```

`Config` 结构体：

| 字段 | 类型 | 说明 |
|------|------|------|
| `Type` | `string` | `"kafka"`, `"rabbitmq"`, `"redis"` |
| `Brokers` | `[]string` | Kafka broker 地址列表 |
| `URL` | `string` | RabbitMQ AMQP URL |
| `Addr` | `string` | Redis 地址 |
| `Password` | `string` | Redis 密码 |
| `DB` | `int` | Redis DB 编号 |

按需注册包：

| Type | 注册包 |
|------|--------|
| `kafka` | `messaging/pubsub/factory/kafka` |
| `rabbitmq` | `messaging/pubsub/factory/rabbitmq` |
| `redis` | `messaging/pubsub/factory/redis` |

### 高级用法（直接使用 driver）

```go
// --- Kafka ---
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"

pub, _ := kafka.NewPublisher(saramaClient)
defer pub.Close()

pub.Publish(ctx, "orders", &pubsub.Message{
    Key:  []byte("order-123"),
    Body: []byte(`{"id":"123"}`),
})

sub, _ := kafka.NewSubscriber(saramaClient, "my-group")
defer sub.Close()

ch, _ := sub.Subscribe(ctx, "orders")
for msg := range ch {
    fmt.Println(string(msg.Body))
    sub.Ack(ctx, msg)
}

// --- RabbitMQ ---
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"

pub, _ := rabbitmq.NewPublisher("amqp://localhost",
    rabbitmq.WithExchange("events", "topic"),
)
pub.Publish(ctx, "order.created", &pubsub.Message{Body: payload})

// --- Redis Streams ---
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"

pub, _ := redis.NewPublisher(redisClient, redis.WithMaxLen(10000, true))
pub.Publish(ctx, "notifications", &pubsub.Message{Body: payload})

sub, _ := redis.NewSubscriber(redisClient,
    redis.WithConsumerGroup("workers", "worker-1"),
)
ch, _ := sub.Subscribe(ctx, "notifications")
```

> 直接使用 driver 可以设置更细粒度的选项（如 RabbitMQ 的交换机类型、Redis 的 MaxLen 等），适用于需要精细控制的场景。
