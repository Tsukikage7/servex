# llm/retrieval/hybrid

`github.com/Tsukikage7/servex/v2/llm/retrieval/hybrid` — 混合检索子包，提供向量召回与 BM25 词法召回通过 RRF（Reciprocal Rank Fusion）融合的能力，常用于 RAG 召回阶段兼顾语义相似度与关键词命中。

## 核心类型

- `Retriever` — 召回器接口，方法为 `Retrieve(ctx, query, topK) ([]rag.RetrievedDoc, error)`
- `HybridRetriever` — 混合召回器，并发两路召回并做 RRF 融合
- `New(vector, lexical, opts...)` — 创建 `HybridRetriever`，默认 `k=60`，两路权重均为 `1.0`
- `WithRRFK(k)` — 设置 RRF 公式中的常数 `k`
- `WithWeights(vec, lex)` — 设置向量路与词法路的权重
- `BM25Retriever` — 内存 BM25 词法召回器（中文按单字切、英文按空格/标点切）
- `NewBM25Retriever(docs, opts...)` — 构造并立即建立索引，默认 `k1=1.5`、`b=0.75`
- `WithBM25Params(k1, b)` — 自定义 BM25 的 `k1`、`b` 参数

## RRF 融合公式

```
score(doc) = vw * 1/(k + vRank) + lw * 1/(k + lRank)
```

其中 `vRank`、`lRank` 为文档在向量路、词法路中的排名（从 0 开始），同一 ID 若两路均命中则两项贡献累加。

`HybridRetriever.Retrieve` 并发调用两路召回、各取 `topK*2` 作为候选池以提升融合质量；一路失败仍返回另一路结果，两路都失败才返回合并后的错误（可用 `errors.Is` 匹配原错误）。

## 使用示例

### BM25 单独使用

```go
import "github.com/Tsukikage7/servex/v2/llm/retrieval/hybrid"

corpus := []rag.Document{
    {ID: "d1", Content: "关于退款的详细说明"},
    {ID: "d2", Content: "订单发货与物流查询"},
}
bm25 := hybrid.NewBM25Retriever(corpus)
docs, err := bm25.Retrieve(ctx, "退款政策", 5)
```

### 组合成混合检索

```go
import "github.com/Tsukikage7/servex/v2/llm/retrieval/hybrid"

// 向量路：任何实现 hybrid.Retriever 的对象，通常是 rag.Pipeline.Retrieve 的适配器.
var vector hybrid.Retriever = myVectorAdapter

// 词法路：内存 BM25.
lexical := hybrid.NewBM25Retriever(corpus)

h := hybrid.New(vector, lexical,
    hybrid.WithRRFK(60),
    hybrid.WithWeights(1.0, 1.0),
)

docs, err := h.Retrieve(ctx, "退款政策", 5)
for _, d := range docs {
    fmt.Printf("[%.4f] %s\n", d.Score, d.ID)
}
```

## 接入 rag.Pipeline

`rag.Pipeline.Retrieve(ctx, question)` 签名没有 topK（topK 在 Config 中）。可通过适配器接入：

```go
type pipelineAdapter struct{ p *rag.Pipeline }

func (a pipelineAdapter) Retrieve(ctx context.Context, query string, _ int) ([]rag.RetrievedDoc, error) {
    return a.p.Retrieve(ctx, query)
}

hybridR := hybrid.New(pipelineAdapter{p: myPipeline}, myBM25, hybrid.WithWeights(1.0, 0.5))
```

> **提示:** BM25 索引在内存中构建，适合 < 10 万文档的中小规模语料。

## 可观测性

本包内置 OpenTelemetry span，无需额外配置，只要调用方设置了全局 TracerProvider 即可自动采集：

- `hybrid.Retrieve` span：属性 `hybrid.query_len / hybrid.top_k / hybrid.rrf_k / hybrid.vec_weight / hybrid.lex_weight / hybrid.hits`
- `bm25.Retrieve` span：属性 `bm25.query_len / bm25.top_k / bm25.docs_count / bm25.hits`
- 调用 `HybridRetriever.Retrieve` 时，bm25 span 作为 hybrid span 的子 span 自动 nested（向量路实现自己未加 span 也不影响）
- 两路都失败时 hybrid span Status=Error；仅一路失败 Status=Ok
