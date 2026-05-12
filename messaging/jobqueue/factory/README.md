# messaging/jobqueue/factory

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
```

## 简介

`messaging/jobqueue/factory` 提供配置驱动的任务队列存储工厂，通过 `StoreConfig` 结构体统一创建 `jobqueue.Store`。

factory 核心包不直接导入 Redis、Kafka、RabbitMQ 或数据库后端，避免业务只使用 factory 时被动拉入未使用的间接依赖。使用哪个后端，就按需 blank import 对应注册包。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `StoreConfig` | 存储配置，`Type` 字段决定后端类型 |
| `NewStore(cfg)` | 根据已注册的后端类型创建 `jobqueue.Store` |
| `RegisterStore(type, creator)` | 注册自定义后端 |

内置注册包：

| Type | 注册包 |
|---|---|
| `redis` | `messaging/jobqueue/factory/redis` |
| `kafka` | `messaging/jobqueue/factory/kafka` |
| `rabbitmq` | `messaging/jobqueue/factory/rabbitmq` |
| `database` | `messaging/jobqueue/factory/database` |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
    _ "github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory/redis"
)

func main() {
    // Redis 后端
    cfg := &factory.StoreConfig{
        Type:     "redis",
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
        Prefix:   "myapp",
    }

    store, err := factory.NewStore(cfg)
    if err != nil {
        panic(err)
    }
    defer store.Close()

    ctx := context.Background()

    // 入队
    job := &jobqueue.Job{
        ID:          "j-001",
        Queue:       "emails",
        Type:        "send_welcome",
        Payload:     []byte(`{"user_id":"u-1"}`),
        Priority:    3,
        ScheduledAt: time.Now(),
    }
    store.Enqueue(ctx, job)

    // 出队
    j, err := store.Dequeue(ctx, "emails")
    if err != nil {
        fmt.Println("队列为空")
        return
    }
    fmt.Printf("任务: %s (type=%s)\n", j.ID, j.Type)
    store.MarkDone(ctx, j.ID)

    // 切换到数据库后端时，改为 blank import factory/database 并更改配置
    _ = &factory.StoreConfig{
        Type:   "database",
        Driver: "postgres",
        DSN:    "host=localhost user=app dbname=mydb",
    }
}
```
