# ai/gemini

Google Gemini REST API 适配器，实现 `llm.ChatModel` 和 `llm.EmbeddingModel` 接口。

## 功能特性

- 非流式与流式聊天生成
- 文本嵌入（EmbedTexts）
- 工具调用（Function Calling）
- 多模态消息（文本 + 图片）
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
| `WithModel(model)` | `gemini-2.0-flash` | 默认聊天模型 |
| `WithEmbeddingModel(model)` | `text-embedding-004` | 默认嵌入模型 |
| `WithBaseURL(url)` | `https://generativelanguage.googleapis.com` | API 基础 URL |
| `WithHTTPClient(hc)` | `http.DefaultClient` | 自定义 HTTP 客户端 |

## 使用示例

```go
client := gemini.New(os.Getenv("GEMINI_API_KEY"),
    gemini.WithModel("gemini-2.0-flash"),
)

resp, err := client.Generate(ctx, []llm.Message{
    llm.UserMessage("用 Go 实现一个简单的 HTTP 服务器"),
})
fmt.Println(resp.Message.Content)
```

## 许可证

详见项目根目录 LICENSE 文件。
