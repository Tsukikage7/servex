# ai/vectorstore/memory

`github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/memory` — 基于内存的向量存储实现，适用于测试和原型开发。

## 功能特性

- 实现 `vectorstore.VectorStore` 接口，可无缝替换为生产级存储
- 余弦相似度计算，结果按分数降序返回
- 支持元数据过滤（`WithFilter`）和分数阈值（`WithScoreThreshold`）
- 并发安全（读写锁保护）
- 零外部依赖

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm
```

## 接口

```go
func New() *Store

func (s *Store) AddDocuments(ctx context.Context, docs []vectorstore.Document) error
func (s *Store) SimilaritySearch(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error)
func (s *Store) Delete(ctx context.Context, ids []string) error
```

## 使用示例

```go
import (
    "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
    "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/memory"
)

store := memory.New()

// 写入文档
_ = store.AddDocuments(ctx, []vectorstore.Document{
    {ID: "1", Content: "Go 并发编程", Vector: vec1, Metadata: map[string]any{"lang": "zh"}},
    {ID: "2", Content: "Concurrency in Go", Vector: vec2, Metadata: map[string]any{"lang": "en"}},
})

// 相似度检索
results, _ := store.SimilaritySearch(ctx, queryVec, 3,
    vectorstore.WithScoreThreshold(0.7),
    vectorstore.WithFilter(map[string]any{"lang": "zh"}),
)
for _, r := range results {
    fmt.Printf("%.4f  %s\n", r.Score, r.Document.Content)
}

// 删除文档
_ = store.Delete(ctx, []string{"1"})
```

## 注意事项

- 数据仅存在于内存，进程退出后丢失，不适合生产环境
- 生产环境请替换为 `pgvector` 或 `redis` 适配器

## 许可证

详见项目根目录 LICENSE 文件。
