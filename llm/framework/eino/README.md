# llm/framework/eino

`github.com/Tsukikage7/servex/v2/llm/framework/eino` 是独立 Go module，负责 servex LLM facade 与 CloudWeGo Eino 的类型适配。

## 边界

- 将 Eino `BaseChatModel` 适配为 servex `llm.ChatModel`。
- 将 servex `llm.ChatModel` 适配为 Eino `BaseChatModel`。
- 将 Eino embedder 适配为 servex `llm.EmbeddingModel`。
- 将 servex `llm.EmbeddingModel` 适配为 Eino embedder。
- 转换消息、工具定义和基础工具调用结构。

复杂 Agent、Graph、RAG、workflow 和框架私有运行时语义由 Eino 原生能力负责，servex 不复刻。

## 验证

```bash
go test ./...
```
