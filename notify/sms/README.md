# notify/sms

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify/sms"
```

## 简介

`notify/sms` 提供短信发送实现，实现 `notify.Sender` 接口。支持阿里云和腾讯云两种短信服务提供商，通过选项函数配置 AccessKey、签名名称和模板参数。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Sender` | 短信发送者，实现 `notify.Sender` |
| `NewSender(opts...)` | 创建短信发送者 |
| `WithProvider(provider)` | 设置提供商（`"aliyun"` 或 `"tencent"`） |
| `WithAccessKeyID(id)` | 设置 AccessKey ID |
| `WithAccessKeySecret(secret)` | 设置 AccessKey Secret |
| `WithSignName(name)` | 设置短信签名名称 |
| `WithTemplateID(id)` | 设置默认短信模板 ID |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify"
    "github.com/Tsukikage7/servex/v2/notify/sms"
)

func main() {
    // 使用阿里云短信服务
    sender := sms.NewSender(
        sms.WithProvider("aliyun"),
        sms.WithAccessKeyID("your-access-key-id"),
        sms.WithAccessKeySecret("your-access-key-secret"),
        sms.WithSignName("我的应用"),
        sms.WithTemplateID("SMS_123456789"),
    )

    ctx := context.Background()

    // 发送验证码短信
    result := sender.Send(ctx, notify.Message{
        Channel: notify.ChannelSMS,
        To:      "+8613800138000",
        Body:    `{"code":"123456","product":"我的应用"}`,
    })

    if result.Success {
        fmt.Println("短信发送成功，消息 ID:", result.MessageID)
    } else {
        fmt.Println("短信发送失败:", result.Error)
    }
}
```
