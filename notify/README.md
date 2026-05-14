# notify

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/notify"
```

## 简介

`notify` 提供统一的通知发送抽象层，支持邮件、短信、Webhook、推送等多种渠道。核心类型包括 `Sender` 接口、`Message` 消息结构、`Dispatcher` 多渠道分发器和 `TemplateEngine` 模板引擎接口。异步 JobQueue 投递在 `notify/jobqueuex` 子包中按需启用。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Channel` | 通知渠道常量（`ChannelEmail/SMS/Webhook/Push`） |
| `Message` | 消息结构（Channel/To/Subject/Body/TemplateID/Data/Metadata） |
| `Result` | 发送结果（MessageID/Channel/Success/Error/SentAt） |
| `Sender` | 发送者接口（`Send(ctx, msg) (*Result, error)`） |
| `TemplateEngine` | 模板引擎接口（`Render(id, data) (string, error)`） |
| `Dispatcher` | 多渠道分发器 |
| `NewDispatcher()` | 创建分发器 |
| `Dispatcher.Register(sender)` | 注册渠道发送者 |
| `Dispatcher.Send(ctx, msg)` | 分发消息到对应渠道 |
| `ValidateMessage(msg)` | 校验消息合法性 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/notify"
)

// MockSender 模拟发送者
type MockSender struct{}

func (s *MockSender) Channel() notify.Channel { return notify.ChannelEmail }
func (s *MockSender) Close() error { return nil }

func (s *MockSender) Send(ctx context.Context, msg *notify.Message) (*notify.Result, error) {
    fmt.Printf("[%s] 发送至 %v: %s\n", msg.Channel, msg.To, msg.Body)
    return &notify.Result{
        Channel: msg.Channel,
        Success: true,
    }, nil
}

func main() {
    dispatcher := notify.NewDispatcher()
    dispatcher.Register(&MockSender{})

    ctx := context.Background()

    // 发送邮件通知
    result, err := dispatcher.Send(ctx, &notify.Message{
        Channel: notify.ChannelEmail,
        To:      []string{"user@example.com"},
        Subject: "订单确认",
        Body:    "您的订单 #12345 已确认。",
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("邮件发送成功:", result.Success)
}
```

### 异步投递

```go
import "github.com/Tsukikage7/servex/v2/notify/jobqueuex"

asyncDispatcher := jobqueuex.NewDispatcher(jobClient)
err := asyncDispatcher.SendAsync(ctx, &notify.Message{
    Channel: notify.ChannelEmail,
    To:      []string{"user@example.com"},
    Body:    "后台异步发送",
})
```
