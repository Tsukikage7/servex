# notify/webhook/store/memory

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/webhook/store/memory"
```

## 简介

`notify/webhook/store/memory` 提供基于内存的 Webhook 订阅存储实现，实现 `webhook.SubscriptionStore` 接口。适用于测试、开发环境或单实例部署场景，所有数据存储在内存中，进程重启后丢失。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Store` | 内存 Webhook 订阅存储，实现 `webhook.SubscriptionStore` |
| `NewStore()` | 创建内存存储 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify/webhook"
    memstore "github.com/Tsukikage7/servex/v2/notify/webhook/store/memory"
)

func main() {
    store := memstore.NewStore()
    ctx := context.Background()

    // 注册 Webhook 订阅
    subs := []*webhook.Subscription{
        {
            ID:     "sub-1",
            URL:    "https://app1.example.com/hooks",
            Events: []string{"user.registered", "order.created"},
            Active: true,
        },
        {
            ID:     "sub-2",
            URL:    "https://app2.example.com/hooks",
            Events: []string{"order.created", "order.shipped"},
            Active: true,
        },
    }
    for _, s := range subs {
        store.Save(ctx, s)
    }

    // 查询 order.created 事件的订阅者
    matched, _ := store.FindByEvent(ctx, "order.created")
    fmt.Printf("共 %d 个订阅者关注 order.created 事件\n", len(matched))
    for _, s := range matched {
        fmt.Println(" -", s.URL)
    }

    // 删除订阅
    store.Delete(ctx, "sub-1")
    fmt.Println("已删除 sub-1")
}
```
