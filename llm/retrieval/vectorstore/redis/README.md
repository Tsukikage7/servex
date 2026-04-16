# ai/vectorstore/redis

`github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/redis` — 基于 Redis Stack（RediSearch）的向量存储实现。

## 功能特性

- 实现 `vectorstore.VectorStore` 接口
- HNSW 近似最近邻索引（COSINE distance）
- `AutoMigrate` 幂等创建 FT 索引（已存在时静默跳过）
- 文档以 Hash 结构存储，key 格式：`{indexName}:{id}`
- 支持元数据过滤和分数阈值

## 前置条件

- [Redis Stack](https://redis.io/docs/stack/) 或带 RediSearch 模块的 Redis 实例

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm
go get github.com/redis/go-redis/v9
```

## 配置

```go
type Config struct {
    Client    *goredis.Client // go-redis 客户端（必填）
    IndexName string          // RediSearch 索引名，同时作为 key 前缀（必填）
    Dimension int             // 向量维度，须 > 0（必填）
}
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
    goredis "github.com/redis/go-redis/v9"
    "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
    vsredis "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/redis"
)

client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})

store, err := vsredis.New(ctx, vsredis.Config{
    Client:    client,
    IndexName: "docs",
    Dimension: 1536,
})
if err != nil {
    log.Fatal(err)
}

// 首次使用：创建索引（幂等）
if err := store.AutoMigrate(ctx); err != nil {
    log.Fatal(err)
}

// 写入文档
_ = store.AddDocuments(ctx, []vectorstore.Document{
    {ID: "doc-1", Content: "Go 并发编程", Vector: vec1536, Metadata: map[string]any{"source": "wiki"}},
})

// 相似度检索
results, _ := store.SimilaritySearch(ctx, queryVec, 5,
    vectorstore.WithScoreThreshold(0.75),
)
for _, r := range results {
    fmt.Printf("%.4f  %s\n", r.Score, r.Document.Content)
}
```

## AutoMigrate 索引结构

```
FT.CREATE docs ON HASH PREFIX 1 docs:
  SCHEMA
    content  TEXT
    metadata TEXT
    vector   VECTOR HNSW 6 TYPE FLOAT32 DIM 1536 DISTANCE_METRIC COSINE
```

## 许可证

详见项目根目录 LICENSE 文件。
