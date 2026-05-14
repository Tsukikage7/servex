# notify/nwebhook

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/nwebhook"
```

## 简介

`notify/nwebhook` 提供 HTTP Webhook 通知发送实现，实现 `notify.Sender` 接口。支持 HMAC 签名验证（防止伪造请求）、自动重试，以及自定义请求头。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Sender` | Webhook 发送者，实现 `notify.Sender` |
| `NewSender(opts...)` | 创建 Webhook 发送者 |
| `WithSecret(secret)` | 设置 HMAC 签名密钥 |
| `WithMaxRetries(n)` | 设置最大重试次数 |
| `WithTimeout(d)` | 设置 HTTP 请求超时时间 |
| `WithHeaders(headers)` | 设置自定义请求头 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify"
    "github.com/Tsukikage7/servex/v2/notify/nwebhook"
)

func main() {
    sender := nwebhook.NewSender(
        nwebhook.WithSecret("my-hmac-secret"),
        nwebhook.WithMaxRetries(3),
    )

    ctx := context.Background()

    // To 字段为 Webhook 回调 URL
    result := sender.Send(ctx, notify.Message{
        Channel: notify.ChannelWebhook,
        To:      "https://partner.example.com/webhook/events",
        Subject: "order.created",
        Body:    `{"order_id":"ord-456","amount":99.99,"currency":"CNY"}`,
    })

    if result.Success {
        fmt.Println("Webhook 推送成功，消息 ID:", result.MessageID)
    } else {
        fmt.Println("Webhook 推送失败:", result.Error)
    }
}
```
