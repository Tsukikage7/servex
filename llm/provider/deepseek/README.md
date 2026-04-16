# llm/provider/deepseek

DeepSeek API 适配器，实现 `llm.ChatModel` 和 `llm.EmbeddingModel` 接口。

底层复用 OpenAI 格式接口（DeepSeek 兼容 OpenAI `/v1` 路径）。

## 功能特性

- 非流式与流式聊天生成
- 文本嵌入（EmbedTexts）
- 多模态消息（文本 + 图片 URL）
- 工具调用（Function Calling）
- 统一错误映射（401/403/429/5xx → 哨兵错误）
- 支持自定义 HTTP 客户端（代理、超时）

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm/provider/deepseek
```

## API

### 构造

```go
func New(apiKey string, opts ...Option) *Client
```

### 配置选项

| 选项 | 默认值 | 说明 |
|---|---|---|
| `WithBaseURL(url)` | `https://api.deepseek.com/v1` | API 基础 URL |
| `WithModel(model)` | `deepseek-chat` | 默认聊天模型 |
| `WithEmbeddingModel(model)` | - | 默认嵌入模型 |
| `WithHTTPClient(hc)` | 60s 超时客户端 | 自定义 HTTP 客户端 |

## 使用示例

### 非流式生成

```go
import (
    "github.com/Tsukikage7/servex/v2/llm"
    "github.com/Tsukikage7/servex/v2/llm/provider/deepseek"
)

client := deepseek.New(os.Getenv("DEEPSEEK_API_KEY"),
    deepseek.WithModel("deepseek-chat"),
)

resp, err := client.Generate(ctx, []llm.Message{
    llm.SystemMessage("你是一个专业助手"),
    llm.UserMessage("解释一下 Go 的 goroutine"),
}, llm.WithTemperature(0.7))
if err != nil {
    return err
}
fmt.Println(resp.Message.Content)
fmt.Printf("tokens: %d\n", resp.Usage.TotalTokens)
```

### 流式生成

```go
reader, err := client.Stream(ctx, []llm.Message{
    llm.UserMessage("写一首关于秋天的诗"),
})
if err != nil {
    return err
}
defer reader.Close()

for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    fmt.Print(chunk.Delta)
}
```

### 使用 DeepSeek-R1（推理模型）

```go
client := deepseek.New(os.Getenv("DEEPSEEK_API_KEY"),
    deepseek.WithModel("deepseek-reasoner"),
)
```

## 许可证

详见项目根目录 LICENSE 文件。
