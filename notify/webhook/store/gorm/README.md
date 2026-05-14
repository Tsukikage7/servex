# notify/webhook/store/gorm

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/webhook/store/gorm"
```

## 简介

`notify/webhook/store/gorm` 提供基于 GORM 的 Webhook 订阅存储实现，实现 `webhook.SubscriptionStore` 接口。将 `Subscription` 数据持久化到关系型数据库，支持按事件类型和租户查询订阅列表。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Store` | GORM Webhook 订阅存储，实现 `webhook.SubscriptionStore` |
| `NewStore(db)` | 基于 `*gorm.DB` 创建存储 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"

    "github.com/Tsukikage7/servex/v2/notify/webhook"
    gormstore "github.com/Tsukikage7/servex/v2/notify/webhook/store/gorm"
)

func main() {
    db, err := gorm.Open(postgres.Open("host=localhost user=postgres dbname=myapp"), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    store := gormstore.NewStore(db)
    ctx := context.Background()

    // 创建订阅
    sub := &webhook.Subscription{
        ID:       "sub-001",
        TenantID: "tenant-a",
        URL:      "https://partner.example.com/webhooks",
        Events:   []string{"order.created", "order.paid"},
        Secret:   "hmac-secret-key",
        Active:   true,
    }
    if err := store.Save(ctx, sub); err != nil {
        panic(err)
    }

    // 查询订阅 order.created 事件的所有订阅者
    subs, err := store.FindByEvent(ctx, "order.created")
    if err != nil {
        panic(err)
    }
    for _, s := range subs {
        fmt.Printf("订阅者: %s -> %s\n", s.ID, s.URL)
    }
}
```
