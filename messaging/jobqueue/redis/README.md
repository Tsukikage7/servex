# messaging/jobqueue/redis

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
```

## 简介

`messaging/jobqueue/redis` 提供基于 Redis 的任务队列存储后端实现，实现 `jobqueue.Store` 接口。使用 Sorted Set 实现延迟和优先级队列，score 由调度时间和优先级共同决定。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Store` | Redis 任务存储，实现 `jobqueue.Store` |
| `NewStore(client, opts...)` | 基于 `*redis.Client` 创建 |
| `NewStoreFromConfig(addr, password, db, prefix)` | 从配置创建 |
| `WithPrefix(prefix)` | 设置 Redis key 前缀 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    jqredis "github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
)

func main() {
    store, err := jqredis.NewStoreFromConfig("localhost:6379", "", 0, "myapp")
    if err != nil {
        panic(err)
    }
    defer store.Close()

    ctx := context.Background()

    // 入队（高优先级任务）
    jobHigh := &jobqueue.Job{
        ID:          "j-high",
        Queue:       "default",
        Type:        "critical_task",
        Payload:     []byte(`{"data":"important"}`),
        Priority:    10, // 越大越优先
        MaxRetries:  3,
        ScheduledAt: time.Now(),
    }

    // 入队（延迟 30 秒执行）
    jobDelayed := &jobqueue.Job{
        ID:          "j-delayed",
        Queue:       "default",
        Type:        "cleanup_task",
        Payload:     []byte(`{}`),
        Priority:    1,
        ScheduledAt: time.Now().Add(30 * time.Second),
    }

    store.Enqueue(ctx, jobHigh)
    store.Enqueue(ctx, jobDelayed)

    // 出队（按优先级和调度时间）
    j, err := store.Dequeue(ctx, "default")
    if err != nil {
        fmt.Println("无可用任务:", err)
        return
    }
    fmt.Println("获取到任务:", j.Type) // critical_task

    store.MarkRunning(ctx, j.ID)
    // 处理...
    store.MarkDone(ctx, j.ID)
}
```
