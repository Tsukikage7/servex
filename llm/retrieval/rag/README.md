# llm/retrieval/rag

`github.com/Tsukikage7/servex/llm/retrieval/rag` — 检索增强生成（RAG）管线，支持文档导入（分块、嵌入、存储）、语义检索及非流式/流式生成。

## 核心类型

- `Pipeline` — RAG 管线主体
- `Config` — 管线配置，必填 ChatModel、EmbeddingModel、VectorStore，可选 Splitter、TopK、ScoreThreshold、PromptTemplate
- `Document` — 待导入文档，包含 ID、Content、Metadata
- `RetrievedDoc` — 检索结果，嵌套 Document 并附加 Score（相似度）
- `Result` — RAG 结果，包含 Answer、Sources（检索文档列表）、Usage
- `New(cfg)` — 创建 RAG 管线，校验必填配置并填充默认值（TopK=5）

## 主要方法

- `Ingest(ctx, docs)` — 导入文档：分块（可选）→ 嵌入 → 存入向量库
- `Retrieve(ctx, question)` — 仅检索，返回相关文档列表
- `Query(ctx, question, opts...)` — 检索增强生成，返回 `*Result`
- `QueryStream(ctx, question, opts...)` — 流式检索增强生成，返回 `llm.StreamReader`

## 使用示例

```go
import "github.com/Tsukikage7/servex/llm/retrieval/rag"

pipeline, err := rag.New(&rag.Config{
    ChatModel:      chatModel,
    EmbeddingModel: embModel,
    VectorStore:    vs,
    TopK:           5,
})
if err != nil {
    log.Fatal(err)
}

// 导入文档
_ = pipeline.Ingest(ctx, []rag.Document{
    {ID: "doc1", Content: "Go 是一种静态类型、编译型语言..."},
})

// 检索增强生成
result, err := pipeline.Query(ctx, "Go 语言有什么特点？")
fmt.Println(result.Answer)
for _, s := range result.Sources {
    fmt.Printf("  来源: %s (score=%.3f)\n", s.ID, s.Score)
}
```

## 引用透传（Citations）

RAG 命中结果自带的 `[]RetrievedDoc` 可通过 `Result.Citations()` 抽取为结构化引用列表，便于前端渲染来源卡片。抽取过程不修改 Sources 本身，按约定的 Metadata key 做类型安全的读取。

抽取约定的 Metadata key（Ingest 时写入，推荐使用包导出的常量 `rag.CitationKeyTitle` / `rag.CitationKeyURL` / `rag.CitationKeyChunkIdx`，避免字符串字面量漂移）:

| Metadata key          | 字段类型                                         | 用途                       |
| --------------------- | ------------------------------------------------ | -------------------------- |
| `citation.title`      | string                                           | 文档标题                   |
| `citation.url`        | string                                           | 来源 URL                   |
| `citation.chunk_idx`  | int / int32 / int64 / float64 / json.Number      | 在原文档内的 chunk 序号    |

> 缺失或类型不符会被忽略。`chunk_idx` 为指针类型，`nil` 表示缺失、`0` 表示第 0 块。

`Snippet` 默认取 `Content` 前 200 runes（按 UTF-8 rune 切分，超出追加省略号）。

Ingest 示例（写入 Metadata）:

```go
doc := rag.Document{
    ID:      "faq-001",
    Content: "VPS 退款流程……",
    Metadata: map[string]any{
        rag.CitationKeyTitle:    "VPS 退款 FAQ",
        rag.CitationKeyURL:      "https://x.com/faq",
        rag.CitationKeyChunkIdx: 0,
    },
}
_ = pipeline.Ingest(ctx, []rag.Document{doc})
```

查询后抽取:

```go
result, _ := pipeline.Query(ctx, "VPS 怎么退款？")
for _, c := range result.Citations() {
    fmt.Printf("[%s] %s (%.2f) %s\n", c.DocID, c.Title, c.Score, c.Snippet)
}
```
