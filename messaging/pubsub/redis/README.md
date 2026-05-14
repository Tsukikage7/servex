# messaging/pubsub/redis

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"
```

## 简介

`messaging/pubsub/redis` 提供基于 Redis Streams 的 `pubsub.Publisher` 和 `pubsub.Subscriber` 实现，底层使用 `github.com/redis/go-redis/v9`。每个 topic 对应一个 Redis Stream，支持 Consumer Group 模式和消息确认。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Publisher` | Redis Streams 消息发布者 |
| `NewPublisher(client, opts...)` | 基于 redis.Cmdable 创建 |
| `NewPublisherFromConfig(addr, password, db, log)` | 从配置创建 |
| `Subscriber` | Redis Streams 消息订阅者 |
| `NewSubscriber(client, groupID, opts...)` | 基于 redis.Cmdable 创建 |
| `NewSubscriberFromConfig(addr, password, db, groupID, log)` | 从配置创建 |
| `Publish(ctx, topic, msgs...)` | 发布到指定 Stream |
| `Subscribe(ctx, topic)` | 以 Consumer Group 订阅 Stream |
| `Ack/Nack` | 确认/拒绝消息 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/messaging/pubsub"
    pubsubRedis "github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

func main() {
    log := logger.NewNop()

    // 创建发布者
    pub, err := pubsubRedis.NewPublisherFromConfig("localhost:6379", "", 0, log)
    if err != nil {
        panic(err)
    }
    defer pub.Close()

    ctx := context.Background()

    // 发布到 Stream
    err = pub.Publish(ctx, "events:orders",
        &pubsub.Message{
            Body: []byte(`{"order_id":"456","status":"created"}`),
            Headers: map[string]string{"version": "1"},
        },
    )
    if err != nil {
        fmt.Println("发布失败:", err)
        return
    }

    // 创建订阅者（Consumer Group: order-processor）
    sub, err := pubsubRedis.NewSubscriberFromConfig(
        "localhost:6379", "", 0, "order-processor", log,
    )
    if err != nil {
        panic(err)
    }
    defer sub.Close()

    ch, _ := sub.Subscribe(ctx, "events:orders")

    msg := <-ch
    fmt.Println("收到:", string(msg.Body))
    sub.Ack(ctx, msg)
}
```
