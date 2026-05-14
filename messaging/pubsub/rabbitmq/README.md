# messaging/pubsub/rabbitmq

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"
```

## 简介

`messaging/pubsub/rabbitmq` 提供基于 RabbitMQ 的 `pubsub.Publisher` 和 `pubsub.Subscriber` 实现，底层使用 `github.com/rabbitmq/amqp091-go`。Publisher 支持发布确认（Confirm 模式），Subscriber 基于 AMQP Queue 消费。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Publisher` | RabbitMQ 消息发布者 |
| `NewPublisher(url, opts...)` | 基于 AMQP URL 创建 |
| `NewPublisherFromConfig(url, log)` | 从配置创建 |
| `Subscriber` | RabbitMQ 消息订阅者 |
| `NewSubscriber(url, opts...)` | 基于 AMQP URL 创建 |
| `NewSubscriberFromConfig(url, log)` | 从配置创建 |
| `Publish(ctx, topic, msgs...)` | 发布消息（topic 作为 routing key） |
| `Subscribe(ctx, topic)` | 订阅队列，返回消息 channel |
| `Ack/Nack` | 确认/拒绝消息 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/messaging/pubsub"
    "github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

func main() {
    log := logger.NewNop()
    amqpURL := "amqp://guest:guest@localhost:5672/"

    // 创建发布者
    pub, err := rabbitmq.NewPublisherFromConfig(amqpURL, log)
    if err != nil {
        panic(err)
    }
    defer pub.Close()

    ctx := context.Background()

    // 发布消息
    err = pub.Publish(ctx, "user.events",
        &pubsub.Message{
            Body: []byte(`{"event":"user_registered"}`),
            Headers: map[string]string{"content-type": "application/json"},
        },
    )
    if err != nil {
        fmt.Println("发布失败:", err)
        return
    }
    fmt.Println("消息已发布")

    // 创建订阅者
    sub, err := rabbitmq.NewSubscriberFromConfig(amqpURL, log)
    if err != nil {
        panic(err)
    }
    defer sub.Close()

    ch, err := sub.Subscribe(ctx, "user.events")
    if err != nil {
        panic(err)
    }

    msg := <-ch
    fmt.Println("收到消息:", string(msg.Body))
    sub.Ack(ctx, msg)
}
```
