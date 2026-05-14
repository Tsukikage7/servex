# messaging/jobqueue/rabbitmq

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/jobqueue/rabbitmq"
```

## 简介

`messaging/jobqueue/rabbitmq` 提供基于 RabbitMQ 的任务队列存储后端实现，实现 `jobqueue.Store` 接口。每个队列对应一个 AMQP 队列，支持持久化（durable）和死信队列（dead letter queue）配置。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Store` | RabbitMQ 任务存储，实现 `jobqueue.Store` |
| `NewStore(conn, opts...)` | 基于 `*amqp.Connection` 创建 |
| `NewStoreFromConfig(url)` | 从 AMQP URL 创建 |
| `WithDurable(durable)` | 设置队列持久化（默认 true） |
| `WithPrefetchCount(n)` | 设置预取数量（默认 1） |
| `WithPrefix(prefix)` | 设置队列名前缀 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue/rabbitmq"
)

func main() {
    amqpURL := "amqp://guest:guest@localhost:5672/"

    store, err := rabbitmq.NewStoreFromConfig(amqpURL)
    if err != nil {
        panic(err)
    }
    defer store.Close()

    ctx := context.Background()

    // 入队任务
    job := &jobqueue.Job{
        ID:          "email-001",
        Queue:       "emails",
        Type:        "send_invoice",
        Payload:     []byte(`{"order_id":"ord-123","email":"user@example.com"}`),
        Priority:    3,
        MaxRetries:  3,
        ScheduledAt: time.Now(),
    }
    if err := store.Enqueue(ctx, job); err != nil {
        panic(err)
    }
    fmt.Println("任务已入队")

    // 出队
    j, err := store.Dequeue(ctx, "emails")
    if err != nil {
        fmt.Println("无任务:", err)
        return
    }
    fmt.Println("处理任务:", j.Type)

    // 处理成功
    store.MarkDone(ctx, j.ID)

    // 处理失败（重试）
    // store.MarkFailed(ctx, j.ID, fmt.Errorf("SMTP error"))
}
```
