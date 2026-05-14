# messaging/jobqueue/database

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/messaging/jobqueue/database"
```

## 简介

`messaging/jobqueue/database` 提供基于 GORM 的任务队列存储后端实现，实现 `jobqueue.Store` 接口。使用关系数据库（MySQL/PostgreSQL/SQLite）持久化任务，支持优先级、延迟调度，自动创建 `jobqueue_jobs` 表。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Store` | 数据库任务存储，实现 `jobqueue.Store` |
| `NewStore(db, opts...)` | 基于 `*gorm.DB` 创建，自动迁移表 |
| `NewStoreFromConfig(driver, dsn, table)` | 从驱动名和 DSN 创建 |
| `WithTableName(name)` | 自定义表名（默认 `jobqueue_jobs`） |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Tsukikage7/servex/v2/messaging/jobqueue"
    "github.com/Tsukikage7/servex/v2/messaging/jobqueue/database"
)

func main() {
    // 使用 SQLite（生产环境建议 MySQL/PostgreSQL）
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    store, err := database.NewStore(db)
    if err != nil {
        panic(err)
    }
    defer store.Close()

    ctx := context.Background()

    // 入队任务
    job := &jobqueue.Job{
        ID:          "job-001",
        Queue:       "default",
        Type:        "send_email",
        Payload:     []byte(`{"to":"user@example.com","subject":"Hello"}`),
        Priority:    5,
        MaxRetries:  3,
        ScheduledAt: time.Now(),
    }
    if err := store.Enqueue(ctx, job); err != nil {
        panic(err)
    }

    // 出队并执行
    j, err := store.Dequeue(ctx, "default")
    if err != nil {
        fmt.Println("无可用任务:", err)
        return
    }
    fmt.Println("处理任务:", j.Type, j.ID)

    // 标记完成
    store.MarkDone(ctx, j.ID)
}
```
