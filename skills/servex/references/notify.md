# servex 通知服务

## 核心类型

```go
import "github.com/Tsukikage7/servex/v2/notify"

// 通知渠道
notify.ChannelEmail   // "email"
notify.ChannelSMS     // "sms"
notify.ChannelWebhook // "webhook"
notify.ChannelPush    // "push"
// 另见子包中定义的渠道：
// telegram.ChannelTelegram  // "telegram"（v2.0.1+）
// discord.ChannelDiscord    // "discord"（v2.0.1+）

// 消息
msg := &notify.Message{
    Channel:      notify.ChannelEmail,
    To:           []string{"user@example.com"},
    Subject:      "验证码",
    Body:         "您的验证码是 123456",
    TemplateID:   "verify_code",              // 可选：使用模板
    TemplateData: map[string]any{"code": "123456"},
    Metadata:     map[string]string{"type": "otp"},
}

// Sender 接口（各渠道实现）
type Sender interface {
    Send(ctx context.Context, msg *Message) (*Result, error)
    Channel() Channel
    Close() error
}

// 模板引擎接口
type TemplateEngine interface {
    Render(templateID string, data map[string]any) (string, error)
}
```

## Dispatcher -- 通知分发器

```go
import "github.com/Tsukikage7/servex/v2/notify"

// 创建分发器
dispatcher := notify.NewDispatcher(
    notify.WithLogger(log),
    notify.WithTemplateEngine(tmplEngine),     // 可选：模板引擎
    notify.WithDefaultChannel(notify.ChannelEmail),
)

// 注册渠道发送器
dispatcher.Register(emailSender)
dispatcher.Register(smsSender)
dispatcher.Register(webhookSender)
dispatcher.Register(pushSender)

// 同步发送
result, err := dispatcher.Send(ctx, &notify.Message{
    Channel: notify.ChannelEmail,
    To:      []string{"user@example.com"},
    Subject: "欢迎注册",
    Body:    "感谢注册我们的服务",
})

// 多渠道广播
results := dispatcher.Broadcast(ctx,
    []notify.Channel{notify.ChannelEmail, notify.ChannelSMS},
    msg,
)

// 关闭（依次关闭所有 Sender）
dispatcher.Close()
```

异步通知通过 `notify/jobqueuex` 显式接入 JobQueue，避免同步通知场景引入队列依赖：

```go
import "github.com/Tsukikage7/servex/v2/notify/jobqueuex"

asyncDispatcher := jobqueuex.NewDispatcher(
    jobClient,
    jobqueuex.WithDefaultChannel(notify.ChannelEmail),
)

err = asyncDispatcher.SendAsync(ctx, msg)
```

## email -- 邮件发送

```go
import "github.com/Tsukikage7/servex/v2/notify/email"

sender, err := email.NewSender(
    email.WithSMTP("smtp.example.com", 587),
    email.WithAuth("user", "pass"),
    email.WithFrom("noreply@example.com", "系统通知"),
    email.WithTLS(true),
    email.WithLogger(log),
)

dispatcher.Register(sender)
```

## sms -- 短信发送

```go
import "github.com/Tsukikage7/servex/v2/notify/sms"

// 阿里云短信
provider := sms.NewAliyunProvider(sms.AliyunConfig{
    AccessKeyID:     "your-key-id",
    AccessKeySecret: "your-key-secret",
    SignName:        "你的签名",
})

// 腾讯云短信
provider := sms.NewTencentProvider(sms.TencentConfig{
    SecretID:  "your-secret-id",
    SecretKey: "your-secret-key",
    AppID:     "your-app-id",
})
```

## push -- 推送通知

```go
import "github.com/Tsukikage7/servex/v2/notify/push"

// Apple APNs
provider := push.NewAPNsProvider(push.APNsConfig{
    BundleID:   "com.example.app",
    TeamID:     "TEAM_ID",
    KeyID:      "KEY_ID",
    KeyFile:    "path/to/AuthKey.p8",
    Production: false,
})

// Firebase FCM
provider := push.NewFCMProvider(push.FCMConfig{
    CredentialsFile: "path/to/serviceAccount.json",
})
```

## nwebhook -- Webhook 通知

```go
import "github.com/Tsukikage7/servex/v2/notify/nwebhook"

// 支持多种消息格式
// Slack / DingTalk / Lark / 自定义

sender := nwebhook.NewSender(
    nwebhook.WithURL("https://hooks.slack.com/services/..."),
    nwebhook.WithFormat("slack"),    // "slack", "dingtalk", "lark", 或自定义
    nwebhook.WithTimeout(10 * time.Second),
)
dispatcher.Register(sender)
```

## telegram -- Telegram 通知（v2.0.1+）

```go
import "github.com/Tsukikage7/servex/v2/notify/telegram"

// 方式一：独立创建（内部自建 BotAPI 连接）
sender, err := telegram.NewSender("YOUR_BOT_TOKEN")
if err != nil { ... }
dispatcher.Register(sender)

// 方式二：复用 botserver/telegram 的 BotAPI 客户端
import tgbot "github.com/Tsukikage7/servex/v2/transport/botserver/telegram"

bot, _ := tgbot.New("YOUR_BOT_TOKEN", ...)
sender := telegram.NewSenderWithClient(bot.Client())
dispatcher.Register(sender)

// 发送消息（msg.To 为 chat ID 字符串列表，msg.Body 为消息正文）
result, err := dispatcher.Send(ctx, &notify.Message{
    Channel: telegram.ChannelTelegram,
    To:      []string{"123456789"},
    Body:    "服务器告警：CPU 使用率超过 90%",
})
```

**关键类型：**
- `telegram.NewSender(token, opts...) (*Sender, error)` — 独立创建（内部创建 BotAPI）
- `telegram.NewSenderWithClient(client *tgbotapi.BotAPI) *Sender` — 复用已有 BotAPI 客户端
- `telegram.ChannelTelegram` — 渠道标识 `"telegram"`
- 错误：`ErrEmptyToken(70071)`, `ErrBotAPICreate(70072)`, `ErrInvalidChatID(70073)`

## discord -- Discord 通知（v2.0.1+）

```go
import "github.com/Tsukikage7/servex/v2/notify/discord"

// 方式一：独立创建（内部创建 Session，仅用 HTTP API，不建立 Gateway 连接）
sender, err := discord.NewSender("YOUR_BOT_TOKEN")
if err != nil { ... }
dispatcher.Register(sender)

// 方式二：复用 botserver/discord 的 Session
import dcbot "github.com/Tsukikage7/servex/v2/transport/botserver/discord"

bot, _ := dcbot.New("YOUR_BOT_TOKEN", ...)
sender := discord.NewSenderWithClient(bot.Session())
dispatcher.Register(sender)

// 发送消息（msg.To 为 channel ID 列表，msg.Body 为消息正文）
result, err := dispatcher.Send(ctx, &notify.Message{
    Channel: discord.ChannelDiscord,
    To:      []string{"1234567890123456789"},
    Body:    "部署完成：v2.0.1 已上线",
})
```

**关键类型：**
- `discord.NewSender(token, opts...) (*Sender, error)` — 独立创建（token 不含 "Bot " 前缀）
- `discord.NewSenderWithClient(session *discordgo.Session) *Sender` — 复用已有 Session
- `discord.ChannelDiscord` — 渠道标识 `"discord"`
- 错误：`ErrEmptyToken(70061)`, `ErrSessionCreate(70062)`

## factory -- 配置驱动工厂

```go
import "github.com/Tsukikage7/servex/v2/notify/factory"

// 从配置文件创建 Dispatcher（自动初始化所有配置的渠道）
cfg := factory.Config{
    DefaultChannel: "email",
    Email: &factory.EmailConfig{
        Host: "smtp.example.com", Port: 587,
        Username: "user", Password: "pass",
        From: "noreply@example.com", Name: "系统通知",
        TLS: true,
    },
    SMS: &factory.SMSConfig{
        Provider: "aliyun",
        SignName: "你的签名",
        Aliyun: &factory.AliyunSMSConfig{
            AccessKeyID:     "key-id",
            AccessKeySecret: "key-secret",
        },
    },
    Webhook: &factory.WebhookConfig{
        Timeout: 10,
        Retry:   3,
    },
    Push: &factory.PushConfig{
        Provider: "fcm",
        FCM: &factory.FCMPushConfig{
            CredentialsFile: "path/to/creds.json",
        },
    },
}

dispatcher, err := factory.NewDispatcher(cfg,
    factory.WithLogger(log),
)
```

## 完整示例

```go
// 初始化
emailSender, _ := email.NewSender(
    email.WithSMTP("smtp.example.com", 587),
    email.WithAuth("user", "pass"),
    email.WithFrom("noreply@example.com", "MyApp"),
    email.WithTLS(true),
)

dispatcher := notify.NewDispatcher(
    notify.WithLogger(log),
    notify.WithDefaultChannel(notify.ChannelEmail),
)
dispatcher.Register(emailSender)
defer dispatcher.Close()

// 发送通知
result, err := dispatcher.Send(ctx, &notify.Message{
    Channel: notify.ChannelEmail,
    To:      []string{"user@example.com"},
    Subject: "验证码",
    Body:    "您的验证码是 123456，5 分钟内有效。",
})
if err != nil {
    log.Error("发送失败", err)
}
fmt.Println("消息ID:", result.MessageID)
```
