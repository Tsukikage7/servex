# llm/adapter/adk

`github.com/Tsukikage7/servex/v2/llm/adapter/adk` 是独立 Go module，负责把 servex `llm.ChatModel` 接入 Google ADK。

## 边界

- 创建并包装基础 ADK agent。
- 创建使用 servex `llm.ChatModel` 的 ADK LLMAgent。
- 将 servex `llm.ChatModel` 适配为 ADK `model.LLM`。
- 创建 ADK Runner，并接入 servex 的轻量 wrapper。
- 转换文本内容、工具定义和基础 function call / function response。

ADK 的 session、memory、artifact、tool runtime 和 agent 执行模型由 ADK 原生能力负责，servex 不复刻。

## 验证

```bash
go test ./...
```
