# messaging/pubsub/kafka

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"
```

## 简介

`messaging/pubsub/kafka` 提供基于 Kafka 的 `pubsub.Publisher` 和 `pubsub.Subscriber` 实现，底层使用 `github.com/IBM/sarama`。Publisher 使用同步生产者，Subscriber 使用 Consumer Group 模式。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Publisher` | Kafka 消息发布者 |
| `NewPublisher(client, opts...)` | 基于已有 sarama.Client 创建 |
| `NewPublisherFromConfig(brokers, log)` | 从 broker 地址创建 |
| `Subscriber` | Kafka 消息订阅者（Consumer Group） |
| `NewSubscriber(client, groupID, opts...)` | 基于已有 sarama.Client 创建 |
| `NewSubscriberFromConfig(brokers, groupID, log)` | 从 broker 地址创建 |
| `Publish(ctx, topic, msgs...)` | 发布消息 |
| `Subscribe(ctx, topic)` | 返回消息 channel |
| `Ack(ctx, msg)` / `Nack(ctx, msg)` | 确认/拒绝消息 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/messaging/pubsub"
    "github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

func main() {
    log := logger.NewNop()
    brokers := []string{"localhost:9092"}

    // 创建发布者
    pub, err := kafka.NewPublisherFromConfig(brokers, log)
    if err != nil {
        panic(err)
    }
    defer pub.Close()

    // 发布消息
    ctx := context.Background()
    err = pub.Publish(ctx, "my-topic",
        &pubsub.Message{
            Key:  []byte("partition-key"),
            Body: []byte(`{"event":"user_created","id":"u-1"}`),
            Headers: map[string]string{"source": "user-service"},
        },
    )
    if err != nil {
        fmt.Println("发布失败:", err)
        return
    }

    // 创建订阅者
    sub, err := kafka.NewSubscriberFromConfig(brokers, "my-group", log)
    if err != nil {
        panic(err)
    }
    defer sub.Close()

    ch, err := sub.Subscribe(ctx, "my-topic")
    if err != nil {
        panic(err)
    }

    // 消费消息
    for msg := range ch {
        fmt.Println("收到:", string(msg.Body))
        sub.Ack(ctx, msg)
    }
}
```
