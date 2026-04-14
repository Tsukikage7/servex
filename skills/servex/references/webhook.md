# servex Webhook

## 核心类型

```go
import "github.com/Tsukikage7/servex/v2/notify/webhook"

type Event struct {
    ID        string
    Type      string    // "order.created", "user.deleted"
    Payload   []byte
    Timestamp time.Time
}

type Subscription struct {
    ID       string
    URL      string
    Secret   string            // 用于 HMAC 签名
    Events   []string          // 订阅的事件类型，空表示全部
    Metadata map[string]string
}
```

## Dispatcher（发送端）

```go
d := webhook.NewDispatcher(
    webhook.WithHTTPClient(httpClient),    // 自定义 HTTP Client
    webhook.WithTimeout(10 * time.Second),
    webhook.WithSigner(customSigner),      // 替换签名算法，默认 HMAC-SHA256
    webhook.WithSignatureHeader("X-Webhook-Signature"), // 默认值
)
defer d.Close()

// 单次投递，自动签名
err := d.Dispatch(ctx, sub, event)

// 如需重试，在应用层组合 middleware/retry
retry.Do(ctx, func() error {
    return d.Dispatch(ctx, sub, event)
}).WithMaxAttempts(5).WithDelay(time.Second).Run()
```

## Receiver（接收端）

```go
r := webhook.NewReceiver(
    webhook.WithSecret("my-secret"),
    webhook.WithReceiverSigner(customSigner), // 替换验签算法
)

http.HandleFunc("/webhook", func(w http.ResponseWriter, req *http.Request) {
    event, err := r.Handle(ctx, req) // 解析请求体 + 验签
    if err != nil {
        http.Error(w, "invalid", http.StatusUnauthorized)
        return
    }
    // event.ID, event.Type, event.Payload
})
```

## Signer

```go
// 默认 HMAC-SHA256
signer := webhook.NewHMACSigner()
sig := signer.Sign(payload, secret)
ok := signer.Verify(payload, secret, sig)

// 自定义签名算法：实现 Signer 接口
type Signer interface {
    Sign(payload []byte, secret string) string
    Verify(payload []byte, secret string, signature string) bool
}
```

## SubscriptionStore

```go
// 内存（开发/测试）
import "github.com/Tsukikage7/servex/v2/webhook/store/memory"
store := memory.NewStore()

// GORM（生产）：接受 database.Database
import (
    webhookgorm "github.com/Tsukikage7/servex/v2/notify/webhook/store/gorm"
    "github.com/Tsukikage7/servex/v2/storage/rdbms"
)
db := database.MustNewDatabase(&database.Config{
    Driver: database.DriverMySQL,
    DSN:    "user:pass@tcp(localhost:3306)/dbname",
}, log)
store, _ := webhookgorm.NewStore(db, webhookgorm.WithTableName("webhook_subscriptions"))

// 使用
store.Save(ctx, sub)
subs, _ := store.ListByEvent(ctx, "order.created")
store.Delete(ctx, id)
```
