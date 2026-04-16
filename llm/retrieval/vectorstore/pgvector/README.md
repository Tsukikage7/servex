# ai/vectorstore/pgvector

`github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/pgvector` — 基于 PostgreSQL pgvector 扩展的向量存储实现。

## 功能特性

- 实现 `vectorstore.VectorStore` 接口
- IVFFlat 近似最近邻索引（cosine distance）
- `AutoMigrate` 幂等建表建索引
- ID 冲突时自动 UPSERT
- 支持元数据过滤（JSONB）和分数阈值

## 前置条件

- PostgreSQL 11+
- 安装 [pgvector](https://github.com/pgvector/pgvector) 扩展：`CREATE EXTENSION IF NOT EXISTS vector;`

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm
go get github.com/jackc/pgx/v5
```

## 配置

```go
type Config struct {
    Pool      *pgxpool.Pool // pgx 连接池（必填）
    Table     string        // 表名，须匹配 ^[a-zA-Z_][a-zA-Z0-9_]*$（必填）
    Dimension int           // 向量维度，须 > 0（必填）
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
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
    "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/pgvector"
)

pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))

store, err := pgvector.New(ctx, pgvector.Config{
    Pool:      pool,
    Table:     "embeddings",
    Dimension: 1536,
})
if err != nil {
    log.Fatal(err)
}

// 首次使用：建表和索引（幂等）
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

## AutoMigrate 建表结构

```sql
CREATE TABLE IF NOT EXISTS embeddings (
    id       TEXT PRIMARY KEY,
    content  TEXT NOT NULL,
    vector   vector(1536) NOT NULL,
    metadata JSONB
);
CREATE INDEX IF NOT EXISTS embeddings_vec_idx ON embeddings
    USING ivfflat (vector vector_cosine_ops) WITH (lists = 100);
```

## 许可证

详见项目根目录 LICENSE 文件。
