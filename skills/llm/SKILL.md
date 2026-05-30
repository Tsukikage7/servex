---
name: llm
description: servex LLM 模块专家。当用户使用 servex 的 llm、llm/adapter/eino、llm/adapter/adk、llm/provider/*、llm/prompt、llm/middleware、llm/gateway/* 时触发。
---

# servex LLM

servex 的 LLM 模块提供稳定 facade、Provider、Middleware、Gateway、MCP、Observability 和 adapter 能力。Eino/ADK adapter 位于独立 Go module，避免根 module 持有这些重依赖。

## 当前边界

- `llm`：`ChatModel`、`EmbeddingModel`、`Message`、`Tool`、`CallOption`、`Usage`、`StreamReader`。
- `llm/adapter/eino`：独立 module，CloudWeGo Eino 双向适配。
- `llm/adapter/adk`：独立 module，Google ADK Agent/LLMAgent/Runner 适配。
- `llm/provider/*`：模型后端适配。
- `llm/middleware`：日志、重试、限流、用量追踪。
- `llm/prompt`：轻量提示词模板。
- `llm/gateway/*`：语义缓存、API Key、计费、ServeX AI Gateway。
- `llm/mcp`：MCP 工具注册、最小权限策略和 `llm.Tool` 转换边界。
- `llm/observability`：OpenTelemetry GenAI 属性和用量记录辅助。

已删除自研运行时：`llm/agent`、`llm/compose`、`llm/retrieval`、`llm/processing`、`llm/eval`、`llm/safety`。

## llm/adapter/eino

```go
model, err := eino.NewChatModel(baseEinoModel)
if err != nil {
    return err
}

resp, err := model.Generate(ctx, []llm.Message{
    llm.UserMessage("你好"),
})
```

核心函数：

- `NewChatModel(base)`：将 Eino `BaseChatModel` 适配为 servex `llm.ChatModel`。
- `AsChatModel(model)`：将 servex `llm.ChatModel` 适配为 Eino `BaseChatModel`。
- `NewEmbeddingModel(base)`：将 Eino `Embedder` 适配为 servex `llm.EmbeddingModel`。
- `AsEmbedder(model)`：将 servex `llm.EmbeddingModel` 适配为 Eino `Embedder`。
- `ToEinoMessage` / `ToEinoMessages`：servex 消息转 Eino 消息。
- `FromEinoMessage`：Eino 消息转 servex 消息。
- `ToEinoTool` / `ToEinoTools`：servex 工具定义转 Eino 工具定义。

## llm/adapter/adk

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

核心函数：

- `NewAgent(cfg)`：创建并包装基础 Google ADK agent。
- `NewLLMAgent(cfg)`：创建使用 servex `llm.ChatModel` 的 Google ADK LLMAgent。
- `WrapAgent(agent)`：包装已有 ADK agent。
- `AsModel(name, model)`：将 servex `llm.ChatModel` 适配为 ADK `model.LLM`。
- `NewRunner(cfg)`：创建 ADK Runner，优先接收 servex Agent wrapper。
- `Agent()`：返回底层 ADK agent。

## 注意

不要在根 module 内重新实现 Agent、Graph、RAG 或工具调用循环；这些能力通过独立 adapter 或业务 runtime 接入，ServeX 负责微服务边界闭环。
根 module 的 `go test ./llm/...` 不包含独立 adapter module；修改 adapter 时需要分别在 `llm/adapter/eino` 或 `llm/adapter/adk` 目录运行 `go test ./...`。
