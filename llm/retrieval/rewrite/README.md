# llm/retrieval/rewrite

`github.com/Tsukikage7/servex/v2/llm/retrieval/rewrite` — RAG 检索前的查询改写，提供两种策略：历史感知改写（HistoryAware）把含代词/省略的问题改写为独立完整句子；HyDE 让 LLM 生成假设性答案用作检索查询，提升语义召回精度。

## 核心类型

- `Rewriter` — 改写器接口，方法为 `Rewrite(ctx, query, history []llm.Message) (string, error)`
- `NewHistoryAwareRewriter(model, opts...) (Rewriter, error)` — 基于对话历史把代词/省略改写为独立完整查询（最近 10 条历史，跳过 system 消息）；`model` 为 nil 时返回 `ErrNilModel`
- `NewHyDERewriter(model, opts...) (Rewriter, error)` — HyDE：让 LLM 生成 2-3 句假设性答案用作向量检索查询；`model` 为 nil 时返回 `ErrNilModel`
- `ErrNilModel` — 构造时 `llm.ChatModel` 为 nil 的哨兵错误
- `WithSystemPrompt(p)` — 自定义 system prompt，支持 `{{.Query}}`、`{{.History}}` 作为模板占位符（字符串替换，不引入 `text/template` 依赖）
- `WithMaxTokens(n)` — 改写输出的最大 token 数（HistoryAware 默认 200；HyDE 默认 300）

## 行为约定

- `query` 为空或 `history` 为空（对 HistoryAware）时，直接返回原 `query`，**不**触发 LLM 调用
- LLM 返回空字符串或仅空白时，返回原 `query`（认为无需改写）
- LLM 调用失败时返回原 `query` + 非 nil `error`（可 `errors.Is` 匹配底层），调用方可选择回退到原 `query`
- `HistoryAware` 仅保留最近 10 条非 system 历史消息，`system` 角色会被跳过；角色映射中文 `user` → `用户`，`assistant` → `助手`
- `HyDE` 当前不使用 `history` 参数（保留以符合 `Rewriter` 接口），调用方传 `nil` 即可

## 使用示例

### HistoryAware：多轮客服对话

```go
import (
    "github.com/Tsukikage7/servex/v2/llm"
    "github.com/Tsukikage7/servex/v2/llm/retrieval/rewrite"
)

r, err := rewrite.NewHistoryAwareRewriter(chatModel)
if err != nil {
    // 通常只在 chatModel 为 nil 时发生（ErrNilModel）
    log.Fatal(err)
}

history := []llm.Message{
    llm.UserMessage("A 产品怎么退款"),
    llm.AssistantMessage("支持 7 天内无理由"),
}
// "那 B 呢" → "B 产品怎么退款" 之类的独立完整句
rewritten, err := r.Rewrite(ctx, "那 B 呢", history)
if err != nil {
    // 改写失败可回退到原 query
    rewritten = "那 B 呢"
}
docs, _ := pipeline.Retrieve(ctx, rewritten)
```

### HyDE：生成假设答案做语义召回

```go
r, err := rewrite.NewHyDERewriter(chatModel,
    rewrite.WithMaxTokens(256),
)
if err != nil {
    log.Fatal(err) // ErrNilModel
}
// 让 LLM 先答一个"假设性答案"，再把答案当检索 query
hypothetical, err := r.Rewrite(ctx, "VPS 可以退款吗", nil)
if err != nil {
    hypothetical = "VPS 可以退款吗"
}
docs, _ := pipeline.Retrieve(ctx, hypothetical)
```

## 性能与适用场景

- **每次改写都多一次 LLM 调用**，会引入额外延迟与 token 成本。建议:
  - 首轮对话（`history` 为空）直接跳过改写 — `HistoryAware` 已内置此短路逻辑
  - 对纯词法/模板化 query（例如 FAQ 关键词搜索），可不改写
  - 高并发场景下考虑把 `WithMaxTokens` 调小以降低延迟
- **HistoryAware**：多轮客服/对话场景首选，能显著缓解代词/省略导致的召回漂移
- **HyDE**：语料专业术语较多且用户 query 偏口语时有效，但会放大 LLM 幻觉；推荐与向量+词法混合检索（见 `llm/retrieval/hybrid`）搭配以降低风险
- 两个 Rewriter 输出都可直接喂给 `rag.Pipeline.Retrieve` 或 `hybrid.HybridRetriever.Retrieve` 作为新 query
