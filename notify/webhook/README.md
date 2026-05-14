# webhook

`github.com/Tsukikage7/servex/v2/webhook` -- Webhook 投递与接收。

## 概述

webhook 包提供 Webhook 事件的发送端（Dispatcher）和接收端（Receiver）能力，内置 HMAC-SHA256 签名验签机制，配合 SubscriptionStore 管理订阅关系，实现完整的 Webhook 事件流。

## 功能特性

- 双向支持：Dispatcher 投递事件，Receiver 接收并验证请求
- HMAC-SHA256 签名：内置签名器，防止请求伪造
- 订阅管理：SubscriptionStore 接口管理订阅方的 URL、Secret 和事件过滤
- 可定制 Header：签名头、事件类型头、事件 ID 头均可配置
- 两种 Store：GORM 和 Memory 实现

## 核心类型

### Event

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `string` | 事件唯一 ID |
| `Type` | `string` | 事件类型 |
| `Payload` | `[]byte` | 事件载荷（JSON） |
| `Timestamp` | `time.Time` | 事件时间戳 |

### Subscription

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `string` | 订阅 ID |
| `URL` | `string` | 回调地址 |
| `Secret` | `string` | 签名密钥 |
| `Events` | `[]string` | 订阅的事件类型列表（空表示全部） |
| `Metadata` | `map[string]string` | 附加元数据 |

## 核心接口

### Dispatcher

| 方法 | 说明 |
|------|------|
| `Dispatch(ctx, sub, event) error` | 向订阅方投递事件 |
| `Close() error` | 关闭 |

### Receiver

| 方法 | 说明 |
|------|------|
| `Handle(ctx, *http.Request) (*Event, error)` | 解析并验证 webhook 请求 |

### Signer

| 方法 | 说明 |
|------|------|
| `Sign(payload []byte, secret string) string` | 对载荷签名 |
| `Verify(payload []byte, secret string, signature string) bool` | 验证签名 |

### SubscriptionStore

| 方法 | 说明 |
|------|------|
| `Save(ctx, sub) error` | 保存订阅 |
| `Delete(ctx, id) error` | 删除订阅 |
| `ListByEvent(ctx, eventType) ([]*Subscription, error)` | 按事件类型查询订阅 |
| `Get(ctx, id) (*Subscription, error)` | 获取单个订阅 |

## 构造函数

| 函数 | 说明 |
|------|------|
| `NewDispatcher(opts ...DispatcherOption) *dispatcher` | 创建 Webhook 投递器 |
| `NewReceiver(opts ...ReceiverOption) *receiver` | 创建 Webhook 接收器 |
| `NewHMACSigner() Signer` | 创建 HMAC-SHA256 签名器 |

## 配置选项

### DispatcherOption

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithHTTPClient(*http.Client)` | 内部创建 | 自定义 HTTP 客户端 |
| `WithTimeout(time.Duration)` | `10s` | 请求超时时间 |
| `WithSigner(Signer)` | HMAC-SHA256 | 自定义签名器 |
| `WithSignatureHeader(string)` | `X-Webhook-Signature` | 签名请求头名称 |

### ReceiverOption

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithReceiverSigner(Signer)` | HMAC-SHA256 | 自定义签名器 |
| `WithSecret(string)` | 空（不验签） | 设置签名密钥，启用验签 |
| `WithReceiverSignatureHeader(string)` | `X-Webhook-Signature` | 签名请求头名称 |

## SubscriptionStore 实现

| 后端 | 包路径 | 构造函数 | 说明 |
|------|--------|----------|------|
| GORM | `webhook/store/gorm` | `NewStore(db database.Database, opts ...Option)` | 接受 `database.Database`，自动迁移 |
| Memory | `webhook/store/memory` | `NewStore()` | 内存实现，用于开发测试 |

## 使用示例

```go
// --- 发送端 ---
import (
    "github.com/Tsukikage7/servex/v2/webhook"
    webhookgorm "github.com/Tsukikage7/servex/v2/webhook/store/gorm"
    memstore "github.com/Tsukikage7/servex/v2/webhook/store/memory"
    "github.com/Tsukikage7/servex/v2/storage/rdbms"
)

// GORM Store（生产环境）：接受 database.Database
db := database.MustNewDatabase(&database.Config{
    Driver: database.DriverMySQL,
    DSN:    "user:pass@tcp(localhost:3306)/dbname",
}, log)
gormStore, _ := webhookgorm.NewStore(db)

// Memory Store（开发测试）
store := memstore.NewStore()
dispatcher := webhook.NewDispatcher(
    webhook.WithTimeout(5 * time.Second),
)

// 注册订阅
store.Save(ctx, &webhook.Subscription{
    ID:     "sub-1",
    URL:    "https://example.com/hook",
    Secret: "my-secret",
    Events: []string{"order.created"},
})

// 投递事件
subs, _ := store.ListByEvent(ctx, "order.created")
for _, sub := range subs {
    dispatcher.Dispatch(ctx, sub, &webhook.Event{
        ID:      "evt-1",
        Type:    "order.created",
        Payload: []byte(`{"order_id":"123"}`),
    })
}

// --- 接收端 ---
receiver := webhook.NewReceiver(
    webhook.WithSecret("my-secret"),
)

http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
    event, err := receiver.Handle(r.Context(), r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    fmt.Printf("收到事件: %s, 载荷: %s\n", event.Type, event.Payload)
    w.WriteHeader(http.StatusOK)
})
```
