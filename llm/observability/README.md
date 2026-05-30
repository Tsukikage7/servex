# llm/observability

`github.com/Tsukikage7/servex/v2/llm/observability` 提供 OpenTelemetry GenAI 属性和用量记录辅助，用于把 LLM 调用纳入 Go 微服务观测体系。

这个包只提供轻量 helper，不绑定 Langfuse、Phoenix 或任何具体观测平台。

## 使用示例

```go
span.SetAttributes(observability.ModelAttributes("openai", "gpt-4o-mini")...)
observability.RecordUsage(span, llm.Usage{
    PromptTokens:     10,
    CompletionTokens: 5,
    TotalTokens:      15,
})
```
