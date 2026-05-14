# notify/factory

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/factory"
```

## 简介

`notify/factory` 提供通知发送者的工厂方法，根据统一配置结构自动创建并注册各渠道的 `Sender` 实例，返回可直接使用的 `Dispatcher`。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Config` | 通知工厂总配置（包含各渠道子配置） |
| `Config.Email` | 邮件渠道配置 |
| `Config.SMS` | 短信渠道配置 |
| `Config.Webhook` | Webhook 渠道配置 |
| `Config.Push` | 推送渠道配置 |
| `NewDispatcher(cfg)` | 根据配置创建并返回 Dispatcher |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify"
    "github.com/Tsukikage7/servex/v2/notify/factory"
)

func main() {
    cfg := factory.Config{
        Email: factory.EmailConfig{
            Host:     "smtp.example.com",
            Port:     587,
            Username: "noreply@example.com",
            Password: "smtp-password",
            From:     "noreply@example.com",
            TLS:      true,
        },
        SMS: factory.SMSConfig{
            Provider:        "aliyun",
            AccessKeyID:     "your-access-key-id",
            AccessKeySecret: "your-access-key-secret",
            SignName:        "MyApp",
        },
    }

    dispatcher, err := factory.NewDispatcher(cfg)
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    result := dispatcher.Send(ctx, notify.Message{
        Channel: notify.ChannelEmail,
        To:      "user@example.com",
        Subject: "密码重置",
        Body:    "点击以下链接重置密码：https://example.com/reset?token=abc123",
    })
    fmt.Println("发送结果:", result.Success)
}
```
