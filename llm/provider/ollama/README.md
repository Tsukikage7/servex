# llm/provider/ollama

Ollama 本地模型适配器，实现 `llm.ChatModel` 和 `llm.EmbeddingModel` 接口。

底层复用 OpenAI 格式接口（Ollama 兼容 `/v1` 路径），默认连接 `http://localhost:11434/v1`，无需 API Key。

## 功能特性

- 非流式与流式聊天生成
- 文本嵌入（EmbedTexts）
- 多模态消息（文本 + 图片 URL）
- 工具调用（Function Calling，需模型支持）
- 默认超时 120 秒（本地模型推理较慢）
- 支持自定义 HTTP 客户端（代理、超时）

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm/provider/ollama
```

## API

### 构造

```go
func New(apiKey string, opts ...Option) *Client
```

`apiKey` 对本地 Ollama 服务通常传空字符串 `""`。

### 配置选项

| 选项 | 默认值 | 说明 |
|---|---|---|
| `WithBaseURL(url)` | `http://localhost:11434/v1` | Ollama 服务地址 |
| `WithModel(model)` | `llama3.2` | 默认聊天模型 |
| `WithEmbeddingModel(model)` | - | 默认嵌入模型 |
| `WithHTTPClient(hc)` | 120s 超时客户端 | 自定义 HTTP 客户端 |

## 使用示例

### 非流式生成

```go
import (
    "github.com/Tsukikage7/servex/v2/llm"
    "github.com/Tsukikage7/servex/v2/llm/provider/ollama"
)

client := ollama.New("",
    ollama.WithModel("llama3.2"),
)

resp, err := client.Generate(ctx, []llm.Message{
    llm.UserMessage("帮我写一首关于秋天的诗"),
})
if err != nil {
    return err
}
fmt.Println(resp.Message.Content)
```

### 流式生成

```go
reader, err := client.Stream(ctx, []llm.Message{
    llm.UserMessage("解释一下 Go 的 channel"),
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
client := ollama.New("",
    ollama.WithEmbeddingModel("nomic-embed-text"),
)

resp, err := client.EmbedTexts(ctx, []string{
    "Go 并发编程",
    "Kubernetes 容器编排",
})
// resp.Embeddings[0] — 第一个文本的向量
```

### 连接远程 Ollama 实例

```go
client := ollama.New("",
    ollama.WithBaseURL("http://192.168.1.100:11434/v1"),
    ollama.WithModel("qwen2.5:7b"),
)
```

## 许可证

详见项目根目录 LICENSE 文件。
