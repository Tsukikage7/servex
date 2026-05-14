# messaging/pubsub/factory

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
```

## 简介

`messaging/pubsub/factory` 提供配置驱动的 Pub/Sub 工厂，通过 `Config` 结构体统一创建 `pubsub.Publisher` 和 `pubsub.Subscriber`。

factory 核心包不直接导入 Kafka、RabbitMQ 或 Redis Streams 后端，避免业务只使用 factory 时被动拉入未使用的间接依赖。使用哪个后端，就按需 blank import 对应注册包。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Config` | 连接配置，`Type` 字段决定使用哪个后端 |
| `NewPublisher(cfg, log)` | 根据配置创建 Publisher |
| `NewSubscriber(cfg, group, log)` | 根据配置创建 Subscriber |
| `RegisterPublisher(type, creator)` | 注册自定义 Publisher 后端 |
| `RegisterSubscriber(type, creator)` | 注册自定义 Subscriber 后端 |

内置注册包：

| Type | 注册包 |
|---|---|
| `redis` | `messaging/pubsub/factory/redis` |
| `kafka` | `messaging/pubsub/factory/kafka` |
| `rabbitmq` | `messaging/pubsub/factory/rabbitmq` |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/messaging/pubsub"
    "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
    _ "github.com/Tsukikage7/servex/v2/messaging/pubsub/factory/redis"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

func main() {
    log := logger.NewNop()

    // Redis Streams 配置
    cfg := &factory.Config{
        Type:     "redis",
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    }

    // 创建 Publisher
    pub, err := factory.NewPublisher(cfg, log)
    if err != nil {
        panic(err)
    }
    defer pub.Close()

    // 发布消息
    ctx := context.Background()
    err = pub.Publish(ctx, "orders", &pubsub.Message{
        Body: []byte(`{"order_id":"123"}`),
        Headers: map[string]string{"content-type": "application/json"},
    })
    if err != nil {
        fmt.Println("发布失败:", err)
    }

    // 创建 Subscriber
    sub, err := factory.NewSubscriber(cfg, "order-service", log)
    if err != nil {
        panic(err)
    }
    defer sub.Close()

    // 订阅
    ch, err := sub.Subscribe(ctx, "orders")
    if err != nil {
        panic(err)
    }

    msg := <-ch
    fmt.Println("收到消息:", string(msg.Body))
    sub.Ack(ctx, msg)
}
```
