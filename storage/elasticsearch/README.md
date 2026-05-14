# storage/elasticsearch

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/storage/elasticsearch"
```

## 简介

`storage/elasticsearch` 提供 Elasticsearch 客户端封装，基于 `elastic/go-elasticsearch/v8` 实现。提供分层接口：`Client` 为顶层入口，`Index` 管理索引，`Document` 进行文档 CRUD，`Search` 执行搜索查询。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Config` | ES 连接配置（Addresses/Username/Password/CloudID） |
| `Client` | ES 客户端接口，提供 `Index(name)` 和 `Search()` |
| `Index` | 索引管理接口（`Create/Delete/Exists/Refresh`） |
| `Document` | 文档操作接口（`Index/Get/Update/Delete/BulkIndex`） |
| `Search` | 搜索接口（`Query(ctx, index, body)`） |
| `NewClient(cfg)` | 创建客户端 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Tsukikage7/servex/v2/storage/elasticsearch"
)

type Product struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

func main() {
    cfg := elasticsearch.Config{
        Addresses: []string{"http://localhost:9200"},
        Username:  "elastic",
        Password:  "changeme",
    }

    client, err := elasticsearch.NewClient(cfg)
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    idx := client.Index("products")

    // 创建索引
    if !idx.Exists(ctx) {
        idx.Create(ctx, map[string]any{
            "mappings": map[string]any{
                "properties": map[string]any{
                    "name":  map[string]any{"type": "text"},
                    "price": map[string]any{"type": "float"},
                },
            },
        })
    }

    // 写入文档
    product := Product{ID: "p-001", Name: "笔记本电脑", Price: 5999.00}
    if err := idx.Document().Index(ctx, product.ID, product); err != nil {
        panic(err)
    }

    // 搜索
    result, err := client.Search().Query(ctx, "products", map[string]any{
        "query": map[string]any{
            "match": map[string]any{"name": "笔记本"},
        },
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("找到 %d 条结果\n", result.Total)
}
```
