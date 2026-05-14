# domain/eventsourcing

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/domain/eventsourcing"
```

## 简介

`domain/eventsourcing` 实现事件溯源（Event Sourcing）模式。通过存储聚合根上发生的所有领域事件来重建聚合状态，而非直接存储当前状态。支持可选的快照机制加速聚合加载，内置乐观并发控制。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Aggregate` | 聚合根接口 |
| `BaseAggregate` | 可嵌入的基础聚合根实现 |
| `Event` | 持久化事件（含 ID、聚合ID/类型、版本、事件类型、数据） |
| `Snapshot` | 聚合快照 |
| `EventStore` | 事件存储接口 |
| `SnapshotStore` | 快照存储接口 |
| `GORMEventStore` | 基于 GORM 的事件存储实现 |
| `GORMSnapshotStore` | 基于 GORM 的快照存储实现 |
| `Repository[T]` | 聚合仓库，泛型参数须实现 Aggregate |
| `NewRepository(store, factory, opts...)` | 创建仓库 |
| `WithSnapshotStore[T](store)` | 配置快照存储 |
| `WithSnapshotEvery[T](n)` | 每 n 个事件自动保存快照 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/domain/eventsourcing"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

type Order struct {
    eventsourcing.BaseAggregate
    Status string
}

func NewOrder(id string) *Order {
    return &Order{BaseAggregate: eventsourcing.NewBaseAggregate(id, "Order")}
}

func (o *Order) ApplyEvent(event eventsourcing.Event) error {
    switch event.EventType {
    case "OrderCreated":
        o.Status = "created"
    case "OrderConfirmed":
        o.Status = "confirmed"
    }
    return nil
}

func (o *Order) Create() error {
    return o.RaiseEvent(o.ApplyEvent, "OrderCreated", map[string]string{"status": "created"})
}

func main() {
    db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    store := eventsourcing.NewGORMEventStore(db)
    store.AutoMigrate()

    repo, _ := eventsourcing.NewRepository(store, func() *Order {
        return &Order{BaseAggregate: eventsourcing.NewBaseAggregate("", "Order")}
    })

    order := NewOrder("order-1")
    order.Create()

    ctx := context.Background()
    repo.Save(ctx, order)

    loaded, err := repo.Load(ctx, "order-1")
    if err != nil {
        panic(err)
    }
    fmt.Println("状态:", loaded.Status)   // created
    fmt.Println("版本:", loaded.Version()) // 1
}
```
