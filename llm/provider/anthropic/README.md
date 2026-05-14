# ai/anthropic

Anthropic Claude API 适配器，实现 `llm.ChatModel` 接口。

## 功能特性

- 非流式与流式聊天生成
- 工具调用（Function Calling）
- 多模态消息（文本 + 图片）
- 自动注入 `anthropic-version` 请求头
- 统一错误映射

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm
```

## API

### 构造

```go
func New(apiKey string, opts ...Option) *Client
```

### 配置选项

| 选项 | 默认值 | 说明 |
|---|---|---|
| `WithModel(model)` | `claude-3-5-sonnet-20241022` | 默认模型 |
| `WithBaseURL(url)` | `https://api.anthropic.com` | API 基础 URL |
| `WithAnthropicVersion(v)` | `2023-06-01` | API 版本头 |
| `WithDefaultMaxTokens(n)` | `4096` | 默认最大 token 数（Anthropic 必须） |
| `WithHTTPClient(hc)` | `http.DefaultClient` | 自定义 HTTP 客户端 |

## 使用示例

```go
client := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"),
    anthropic.WithModel("claude-opus-4-6"),
    anthropic.WithDefaultMaxTokens(8192),
)

resp, err := client.Generate(ctx, []llm.Message{
    llm.SystemMessage("你是一个代码审查专家"),
    llm.UserMessage("帮我审查以下 Go 代码：\n```go\n...```"),
})
fmt.Println(resp.Message.Content)
```

## 许可证

详见项目根目录 LICENSE 文件。
