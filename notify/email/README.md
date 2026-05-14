# notify/email

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/email"
```

## 简介

`notify/email` 提供基于 SMTP 的邮件发送实现，实现 `notify.Sender` 接口。支持 TLS 连接、自定义发件人地址，以及通过选项函数配置 SMTP 服务器参数。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Sender` | 邮件发送者，实现 `notify.Sender` |
| `NewSender(opts...)` | 创建邮件发送者 |
| `WithHost(host)` | 设置 SMTP 服务器地址 |
| `WithPort(port)` | 设置 SMTP 端口（默认 587） |
| `WithUsername(user)` | 设置 SMTP 用户名 |
| `WithPassword(pass)` | 设置 SMTP 密码 |
| `WithFrom(addr)` | 设置发件人地址 |
| `WithTLS(enabled)` | 是否启用 TLS |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify"
    "github.com/Tsukikage7/servex/v2/notify/email"
)

func main() {
    sender := email.NewSender(
        email.WithHost("smtp.example.com"),
        email.WithPort(587),
        email.WithUsername("noreply@example.com"),
        email.WithPassword("smtp-password"),
        email.WithFrom("noreply@example.com"),
        email.WithTLS(true),
    )

    ctx := context.Background()

    result := sender.Send(ctx, notify.Message{
        Channel: notify.ChannelEmail,
        To:      "user@example.com",
        Subject: "欢迎注册",
        Body:    "<h1>欢迎</h1><p>感谢您的注册！</p>",
    })

    if result.Success {
        fmt.Println("邮件发送成功，消息 ID:", result.MessageID)
    } else {
        fmt.Println("邮件发送失败:", result.Error)
    }
}
```
