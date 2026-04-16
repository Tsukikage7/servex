# llm/retrieval/vectorstore/elasticsearch

`github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/elasticsearch` — 基于 Elasticsearch kNN 的向量存储实现，支持纯向量搜索和混合搜索（BM25 + kNN）。

## 功能特性

- 实现 `vectorstore.VectorStore` 接口，可无缝替换为其他存储适配器
- 基于 `dense_vector` 字段的 kNN 近似最近邻搜索（cosine 相似度）
- 混合搜索：通过 `WithTextQuery` 将 BM25 文本查询与向量搜索结合
- 支持元数据过滤（`WithFilter`）和分数阈值（`WithScoreThreshold`）
- Bulk API 批量写入和删除，性能高效
- `AutoMigrate` 幂等创建索引（索引已存在时自动忽略）

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm
```

## 接口

```go
func New(ctx context.Context, cfg Config) (*Store, error)

func (s *Store) AutoMigrate(ctx context.Context) error
func (s *Store) AddDocuments(ctx context.Context, docs []vectorstore.Document) error
func (s *Store) SimilaritySearch(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error)
func (s *Store) Delete(ctx context.Context, ids []string) error
```

## 使用示例

```go
import (
    esv8 "github.com/elastic/go-elasticsearch/v8"
    "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
    esvs "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/elasticsearch"
)

client, _ := esv8.NewDefaultClient()

store, err := esvs.New(ctx, esvs.Config{
    Client:    client,
    IndexName: "my_docs",
    Dimension: 1536,
})
// 初始化索引（幂等）
_ = store.AutoMigrate(ctx)

// 写入文档
_ = store.AddDocuments(ctx, []vectorstore.Document{
    {ID: "1", Content: "Go 并发编程", Vector: vec1, Metadata: map[string]any{"lang": "zh"}},
    {ID: "2", Content: "Concurrency in Go", Vector: vec2, Metadata: map[string]any{"lang": "en"}},
})

// 纯向量搜索
results, _ := store.SimilaritySearch(ctx, queryVec, 5,
    vectorstore.WithScoreThreshold(0.7),
    vectorstore.WithFilter(map[string]any{"lang": "zh"}),
)

// 混合搜索（BM25 + kNN）
results, _ = store.SimilaritySearch(ctx, queryVec, 5,
    vectorstore.WithTextQuery("Go 并发"),
)

for _, r := range results {
    fmt.Printf("%.4f  %s\n", r.Score, r.Document.Content)
}

// 删除文档
_ = store.Delete(ctx, []string{"1"})
```

## 注意事项

- Elasticsearch 采用近实时索引，写入后需等待约 1 秒才可搜索到（可通过 `refresh=true` 参数强制刷新）
- 混合搜索时 `_score` 为向量分数与 BM25 分数的加权融合，范围不再是 `[0,1]`
- `Dimension` 必须与实际嵌入向量维度一致，创建索引后不可修改

## 许可证

详见项目根目录 LICENSE 文件。
