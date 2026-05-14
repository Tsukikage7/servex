# notify/push

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/push"
```

## 简介

`notify/push` 提供移动端推送通知发送实现，实现 `notify.Sender` 接口。支持 FCM（Firebase Cloud Messaging）和 APNs（Apple Push Notification service）两种推送提供商。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Sender` | 推送发送者，实现 `notify.Sender` |
| `NewSender(opts...)` | 创建推送发送者 |
| `WithFCMKey(key)` | 设置 FCM 服务器密钥（Android） |
| `WithAPNsCert(certFile, keyFile)` | 设置 APNs 证书（iOS） |
| `WithAPNsBundleID(id)` | 设置 iOS App Bundle ID |
| `WithProvider(provider)` | 设置推送提供商（`"fcm"` 或 `"apns"`） |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify"
    "github.com/Tsukikage7/servex/v2/notify/push"
)

func main() {
    // FCM 推送（Android）
    fcmSender := push.NewSender(
        push.WithProvider("fcm"),
        push.WithFCMKey("AAAAxxxxxxx:APA91bxxxxxx"),
    )

    ctx := context.Background()

    // To 字段为设备 Token
    result := fcmSender.Send(ctx, notify.Message{
        Channel: notify.ChannelPush,
        To:      "device-fcm-token-here",
        Subject: "新消息",
        Body:    "您有一条新消息，请查看。",
    })

    if result.Success {
        fmt.Println("推送发送成功")
    } else {
        fmt.Println("推送发送失败:", result.Error)
    }

    // APNs 推送（iOS）
    apnsSender := push.NewSender(
        push.WithProvider("apns"),
        push.WithAPNsCert("/path/to/cert.pem", "/path/to/key.pem"),
        push.WithAPNsBundleID("com.example.myapp"),
    )

    result = apnsSender.Send(ctx, notify.Message{
        Channel: notify.ChannelPush,
        To:      "ios-device-token-here",
        Subject: "订单更新",
        Body:    "您的订单已发货！",
    })
    fmt.Println("iOS 推送成功:", result.Success)
}
```
