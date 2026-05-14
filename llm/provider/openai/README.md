# ai/openai

OpenAI API 适配器，实现 `llm.ChatModel` 和 `llm.EmbeddingModel` 接口。

兼容所有遵循 OpenAI 接口格式的 Provider：**DeepSeek、通义千问（Qwen/DashScope）、Azure OpenAI** 等，只需修改 `WithBaseURL`。

## 功能特性

- 非流式与流式聊天生成
- 文本嵌入（EmbedTexts）
- 多模态消息（文本 + 图片 URL）
- 工具调用（Function Calling）
- 统一错误映射（401/403/429/5xx → 哨兵错误）
- 支持自定义 HTTP 客户端（代理、超时）

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
| `WithBaseURL(url)` | `https://api.openai.com/v1` | API 基础 URL |
| `WithModel(model)` | `gpt-4o` | 默认聊天模型 |
| `WithEmbeddingModel(model)` | `text-embedding-3-small` | 默认嵌入模型 |
| `WithOrganization(orgID)` | - | OpenAI 组织 ID |
| `WithHTTPClient(hc)` | `http.DefaultClient` | 自定义 HTTP 客户端 |

## 使用示例

### 非流式生成

```go
client := openllm.New(os.Getenv("OPENAI_API_KEY"),
    openllm.WithModel("gpt-4o"),
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

### 文本嵌入

```go
resp, err := client.EmbedTexts(ctx, []string{
    "Go 并发编程",
    "Kubernetes 容器编排",
}, llm.WithModel("text-embedding-3-large"))
// resp.Embeddings[0] — 第一个文本的向量
```

### 接入兼容 OpenAI 格式的第三方 Provider

```go
// DashScope（通义千问）
client := openllm.New(os.Getenv("DASHSCOPE_API_KEY"),
    openllm.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
    openllm.WithModel("qwen-plus"),
)

// DeepSeek
client := openllm.New(os.Getenv("DEEPSEEK_API_KEY"),
    openllm.WithBaseURL("https://api.deepseek.com/v1"),
    openllm.WithModel("deepseek-chat"),
)
```

## 许可证

详见项目根目录 LICENSE 文件。
