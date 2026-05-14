# messaging/jobqueue/kafka

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/jobqueue/kafka"
```

## 简介

`messaging/jobqueue/kafka` 提供基于 Kafka 的任务队列存储后端实现，实现 `jobqueue.Store` 接口。每个队列对应一个 Kafka topic，支持 topic 前缀配置。由于 Kafka 的消费特性，出队（Dequeue）需通过 Consumer Group 方式拉取。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Store` | Kafka 任务存储，实现 `jobqueue.Store` |
| `NewStore(client, opts...)` | 基于 sarama.Client 创建 |
| `NewStoreFromConfig(brokers, prefix)` | 从 broker 地址和前缀创建 |
| `WithPrefix(prefix)` | 设置 topic 前缀（如 `"myapp"` → topic 为 `"myapp.emails"`） |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue/kafka"
)

func main() {
    brokers := []string{"localhost:9092"}

    store, err := kafka.NewStoreFromConfig(brokers, "myapp")
    if err != nil {
        panic(err)
    }
    defer store.Close()

    ctx := context.Background()

    // 入队任务（发布到 Kafka topic "myapp.notifications"）
    job := &jobqueue.Job{
        ID:          "notif-001",
        Queue:       "notifications",
        Type:        "push_notification",
        Payload:     []byte(`{"device_token":"xxx","title":"新消息"}`),
        Priority:    5,
        MaxRetries:  2,
        ScheduledAt: time.Now(),
    }
    if err := store.Enqueue(ctx, job); err != nil {
        panic(err)
    }
    fmt.Println("任务已入队:", job.ID)

    // 注意：Kafka 后端的 Dequeue 通过 Consumer Group 拉取
    // 建议配合 jobqueue.Worker 使用
    worker := jobqueue.NewWorker(store)
    worker.Register("push_notification", func(ctx context.Context, j *jobqueue.Job) error {
        fmt.Println("处理推送任务:", string(j.Payload))
        return nil
    })

    // 启动 Worker（阻塞运行）
    // worker.Start(ctx)
}
```
