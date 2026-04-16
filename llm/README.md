# ai

`ai` 包提供 LLM/AI 服务的统一客户端抽象，支持多 Provider（OpenAI、Anthropic、Gemini 等），所有适配器仅依赖标准库。

## 功能特性

- 统一 `ChatModel` / `EmbeddingModel` 接口，Provider 可随时替换
- 流式与非流式生成，支持工具调用（Function Calling）
- 多模态消息（文本 + 图片）
- 统一错误类型与可重试判断
- 子包覆盖：Provider 适配、中间件链、多轮会话、工具调用、提示词模板、嵌入向量、向量存储接口、多 Provider 路由

## 安装

```bash
go get github.com/Tsukikage7/servex/llm
```

## 子包总览

| 子包 | 说明 |
|---|---|
| `ai/openai` | OpenAI 适配器（兼容 DeepSeek、通义千问等 OpenAI 格式 Provider） |
| `ai/ollama` | Ollama 本地模型适配器（默认 http://localhost:11434/v1） |
| `ai/deepseek` | DeepSeek API 适配器（https://api.deepseek.com/v1） |
| `ai/anthropic` | Anthropic Claude 适配器 |
| `ai/bedrock` | AWS Bedrock 适配器（Converse API，支持 Claude/Titan/Llama/Mistral 等） |
| `ai/gemini` | Google Gemini 适配器 |
| `ai/middleware` | 中间件链（日志、重试、限流、用量追踪） |
| `ai/conversation` | 多轮对话会话管理（BufferMemory / WindowMemory） |
| `ai/toolcall` | 工具注册与自动循环执行器（ReAct 模式） |
| `ai/prompt` | 基于 `text/template` 的提示词模板引擎 |
| `ai/embedding` | 批量嵌入 + 余弦相似度工具函数 |
| `ai/vectorstore` | 向量存储统一接口抽象 |
| `ai/router` | 多 Provider 路由器（按模型名路由） |
| `ai/compose` | DAG 编排引擎（Graph、四范式节点、条件边、Runnable） |
| `ai/vectorstore/memory` | 内存向量存储（测试用） |
| `ai/vectorstore/pgvector` | pgvector 向量存储适配 |
| `ai/vectorstore/redis` | Redis Search 向量存储适配 |

## 核心接口

```go
// 聊天模型
type ChatModel interface {
    Generate(ctx context.Context, messages []Message, opts ...CallOption) (*ChatResponse, error)
    Stream(ctx context.Context, messages []Message, opts ...CallOption) (StreamReader, error)
}

// 嵌入模型
type EmbeddingModel interface {
    EmbedTexts(ctx context.Context, texts []string, opts ...CallOption) (*EmbedResponse, error)
}
```

## 消息构造辅助函数

```go
llm.SystemMessage("你是一个专业助手")
llm.UserMessage("帮我写一首诗")
llm.AssistantMessage("好的，...")
llm.ToolResultMessage(callID, `{"result": "ok"}`)
```

## 调用选项

```go
llm.WithModel("gpt-4o")
llm.WithTemperature(0.7)
llm.WithMaxTokens(1024)
llm.WithTopP(0.9)
llm.WithStop("END")
llm.WithTools(tool1, tool2)
llm.WithToolChoice(llm.ToolChoiceAuto)
llm.WithStreamCallback(fn)
```

## 错误处理

```go
// 哨兵错误
llm.ErrRateLimited          // HTTP 429
llm.ErrContextLength        // 上下文超长
llm.ErrInvalidAuth          // 认证失败
llm.ErrProviderUnavailable  // 服务不可用（5xx）
llm.ErrContentFiltered      // 内容过滤

// 可重试判断
if llm.IsRetryable(err) { ... }

// 获取详细错误信息
var apiErr *llm.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.Provider, apiErr.Message)
}
```

## 快速示例

```go
client := openllm.New(os.Getenv("OPENAI_API_KEY"),
    openllm.WithModel("gpt-4o"),
)

resp, err := client.Generate(ctx, []llm.Message{
    llm.UserMessage("你好"),
})
fmt.Println(resp.Message.Content)
```

## 许可证

详见项目根目录 LICENSE 文件。
