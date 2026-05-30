# llm

`llm` 是 servex 面向 AI 应用的 Go 微服务接入层。它提供稳定的 LLM facade、Provider、Middleware、Gateway 和框架适配能力，让业务可以把 Agent、RAG、Workflow 等 AI 应用纳入 Go 服务、网关、鉴权、计费、限流和可观测性体系。

`llm` 不维护自研 Agent、Graph、RAG、工具调用循环或内容审核 runtime；复杂 AI 应用编排统一交给开源框架。Eino/ADK 封装以独立 Go module 形式放在 `llm/adapter/*`，避免污染根 module 依赖面。

## 分层

1. Core facade
   - `llm.ChatModel`
   - `llm.EmbeddingModel`
   - Message / Tool / Usage / Error / CallOption / StreamReader

2. Providers
   - OpenAI、Anthropic、Gemini、DeepSeek、Ollama、Bedrock 等模型后端。
   - Provider 只负责协议适配和错误归一，不承载业务编排。

3. Production governance
   - `llm/middleware`：日志、重试、限流、用量追踪。
   - `llm/router`：Go 内部模型路由与 fallback。
   - `llm/gateway`：ServeX AI Gateway。
   - `llm/gateway/apikey`、`billing`、`cache`：API Key、计费、语义缓存。
   - `llm/prompt`：轻量提示词模板和版本管理。
   - `llm/mcp`：MCP 工具注册、策略和 `llm.Tool` 转换边界。
   - `llm/observability`：OpenTelemetry GenAI 属性和用量记录辅助。

4. Framework adapters
   - `llm/adapter/eino`：CloudWeGo Eino 适配。
   - `llm/adapter/adk`：Google ADK 适配。
   - Agent、Graph、RAG、Session、Memory、Tool runtime 仍由原框架负责。

## 怎么选

| 场景 | 推荐包 |
| --- | --- |
| 只调用模型 | `llm/provider/openai`、`anthropic`、`gemini`、`deepseek`、`ollama` |
| 多模型路由 | `llm/router` |
| 内部服务加重试/日志/限流 | `llm/middleware` |
| 对外暴露 ServeX AI Gateway | `llm/gateway` |
| API Key 和计费 | `llm/gateway/apikey`、`llm/gateway/billing` |
| 语义缓存 | `llm/gateway/cache` |
| 提示词版本管理 | `llm/prompt` |
| MCP 工具接入 | `llm/mcp` |
| AI 调用观测 | `llm/observability` |
| Go Agent 编排 | `llm/adapter/eino` 或 `llm/adapter/adk` |

## 当前边界

- `llm` 根包：`ChatModel`、`EmbeddingModel`、消息、工具、流、调用选项、用量和错误类型。
- `llm/adapter/eino`：独立 module，CloudWeGo Eino 封装，负责 servex 消息、工具、ChatModel、EmbeddingModel 与 Eino 类型双向适配。
- `llm/adapter/adk`：独立 module，Google ADK 封装，负责 ADK Agent/LLMAgent/Runner 创建，并把 servex `ChatModel` 接入 ADK `model.LLM`。
- `llm/provider/*`：OpenAI、Anthropic、Gemini、Ollama、DeepSeek、Bedrock 等模型后端适配。
- `llm/middleware`：日志、重试、限流、用量追踪。
- `llm/prompt`：轻量提示词模板与版本管理。
- `llm/gateway/*`：语义缓存、API Key、计费、ServeX AI 网关。
- `llm/mcp`：MCP 工具注册、最小权限策略和 `llm.Tool` 转换；不实现 agent loop。
- `llm/observability`：OpenTelemetry GenAI 属性辅助；不绑定具体观测平台。

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

模块路径：`github.com/Tsukikage7/servex/v2/llm/adapter/eino`。

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

`llm/adapter/eino` 还提供 `NewEmbeddingModel`、`AsEmbedder`、`ToEinoMessage`、`ToEinoMessages`、`FromEinoMessage`、`ToEinoTool` 和 `ToEinoTools`，用于边界层显式转换。工具 schema 或消息结构转换失败会直接返回错误，不会静默丢弃工具或降级成普通文本。

## ADK 封装

模块路径：`github.com/Tsukikage7/servex/v2/llm/adapter/adk`。

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

`llm/adapter/adk` 同时提供 `NewAgent`、`WrapAgent`、`AsModel` 和 `NewRunner`。ADK 的 session、memory、artifact、tool runtime 仍由 Google ADK 自身负责；servex 只负责把 `llm.ChatModel`、基础配置和创建边界接进去。

## 稳定性说明

- `llm` 根包、provider、middleware、prompt 和 gateway 基础能力按稳定 API 设计。
- `llm/adapter/eino` 和 `llm/adapter/adk` 是独立 Go module 的开源框架适配层，当前覆盖文本、工具定义和基础工具调用桥接；多模态、框架私有事件和高级 runtime 语义应优先使用 Eino/ADK 原生 API。
- `llm/gateway` 是 ServeX AI 网关，当前覆盖文本聊天、SSE、模型列表、鉴权、计费和内容审核入口；工具流式增量、多模态、response_format 等高级字段需要按业务场景继续扩展。
- 根 module 的 `go.mod` 不包含 Eino/ADK 依赖；只使用 provider/middleware/gateway 的用户不会被迫拉取 adapter 依赖。
- 本仓库本地开发通过根目录 `go.work` 同时加载根 module、`llm/adapter/eino` 和 `llm/adapter/adk`。

## 设计原则

- 不在 servex 内复刻 Eino 或 ADK 的 Agent/Graph/RAG runtime。
- Eino/ADK 的源码引用只存在于 `llm/adapter/eino` 和 `llm/adapter/adk` 独立 module。
- 业务代码需要编排时显式选择 Eino 或 ADK；只使用 provider/middleware/gateway 时不需要接触 adapter 包。

## 验证

```bash
go test ./llm/...
(cd llm/adapter/eino && go test ./...)
(cd llm/adapter/adk && go test ./...)
```
