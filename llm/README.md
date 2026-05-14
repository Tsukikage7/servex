# llm

`llm` 提供 servex 的 LLM 集成门面。模块不再维护自研 Agent、Graph、RAG、工具调用循环或内容审核框架；复杂 AI 应用编排统一交给开源框架，servex 只提供稳定接口、Provider、Middleware、Serving 能力。Eino/ADK 封装以独立 Go module 形式放在 `llm/framework/*`，避免污染根 module 依赖面。

## 当前边界

- `llm` 根包：`ChatModel`、`EmbeddingModel`、消息、工具、流、调用选项、用量和错误类型。
- `llm/framework/eino`：独立 module，CloudWeGo Eino 封装，负责 servex 消息、工具、ChatModel、EmbeddingModel 与 Eino 类型双向适配。
- `llm/framework/adk`：独立 module，Google ADK 封装，负责 ADK Agent/LLMAgent/Runner 创建，并把 servex `ChatModel` 接入 ADK `model.LLM`。
- `llm/provider/*`：OpenAI、Anthropic、Gemini、Ollama、DeepSeek、Bedrock 等模型后端适配。
- `llm/middleware`：日志、重试、限流、用量追踪。
- `llm/prompt`：轻量提示词模板与版本管理。
- `llm/serving/*`：语义缓存、API Key、计费、OpenAI 兼容的最小 Chat Completions 代理。

以下自研运行时已删除：`llm/agent`、`llm/compose`、`llm/retrieval`、`llm/processing`、`llm/eval`、`llm/safety`。

## 核心接口

```go
type ChatModel interface {
    Generate(ctx context.Context, messages []Message, opts ...CallOption) (*ChatResponse, error)
    Stream(ctx context.Context, messages []Message, opts ...CallOption) (StreamReader, error)
}

type EmbeddingModel interface {
    EmbedTexts(ctx context.Context, texts []string, opts ...CallOption) (*EmbedResponse, error)
}
```

## Eino 封装

模块路径：`github.com/Tsukikage7/servex/v2/llm/framework/eino`。

```go
base := /* github.com/cloudwego/eino/components/model.BaseChatModel */

model, err := eino.NewChatModel(base)
if err != nil {
    return err
}

resp, err := model.Generate(ctx, []llm.Message{
    llm.UserMessage("你好"),
})

einoModel, err := eino.AsChatModel(servexModel)
if err != nil {
    return err
}
_ = einoModel
```

`llm/framework/eino` 还提供 `NewEmbeddingModel`、`AsEmbedder`、`ToEinoMessage`、`ToEinoMessages`、`FromEinoMessage`、`ToEinoTool` 和 `ToEinoTools`，用于边界层显式转换。工具 schema 或消息结构转换失败会直接返回错误，不会静默丢弃工具或降级成普通文本。

## ADK 封装

模块路径：`github.com/Tsukikage7/servex/v2/llm/framework/adk`。

```go
agent, err := adk.NewLLMAgent(adk.LLMAgentConfig{
    Name:        "assistant",
    Description: "通用助手",
    Instruction: "回答要简洁",
    Model:       servexModel,
})
if err != nil {
    return err
}

raw := agent.Agent()
_ = raw
```

`llm/framework/adk` 同时提供 `NewAgent`、`WrapAgent`、`AsModel` 和 `NewRunner`。ADK 的 session、memory、artifact、tool runtime 仍由 Google ADK 自身负责；servex 只负责把 `llm.ChatModel`、基础配置和创建边界接进去。

## 稳定性说明

- `llm` 根包、provider、middleware、prompt 和 serving 基础能力按稳定 API 设计。
- `llm/framework/eino` 和 `llm/framework/adk` 是独立 Go module 的开源框架适配层，当前覆盖文本、工具定义和基础工具调用桥接；多模态、框架私有事件和高级 runtime 语义应优先使用 Eino/ADK 原生 API。
- `llm/serving/proxy` 是最小 OpenAI Chat Completions 代理，不承诺完整 OpenAI API 覆盖；工具流式增量、多模态、response_format 等高级字段需要按业务场景继续扩展。
- 根 module 的 `go.mod` 不包含 Eino/ADK 依赖；只使用 provider/middleware/serving 的用户不会被迫拉取 framework 依赖。
- 本仓库本地开发通过根目录 `go.work` 同时加载根 module、`llm/framework/eino` 和 `llm/framework/adk`。

## 设计原则

- 不在 servex 内复刻 Eino 或 ADK 的 Agent/Graph/RAG runtime。
- Eino/ADK 的源码引用只存在于 `llm/framework/eino` 和 `llm/framework/adk` 独立 module。
- 业务代码需要编排时显式选择 Eino 或 ADK；只使用 provider/middleware/serving 时不需要接触 framework 包。

## 验证

```bash
go test ./llm/...
(cd llm/framework/eino && go test ./...)
(cd llm/framework/adk && go test ./...)
```
