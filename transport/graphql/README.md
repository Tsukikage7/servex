# transport/graphql

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/transport/graphql"
```

## 简介

`transport/graphql` 提供 GraphQL HTTP 服务器适配器，基于 `graphql-go/graphql` 实现。提供中间件链机制用于在 resolver 执行前后插入横切逻辑（认证、日志、恢复等），并内置日志和 panic 恢复中间件。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Config` | GraphQL 服务器配置（Schema/Endpoint/Playground） |
| `Server` | GraphQL HTTP 服务器 |
| `New(cfg)` | 创建服务器 |
| `Server.Handler()` | 返回 `http.Handler` |
| `Middleware` | Resolver 中间件类型 |
| `ChainMiddleware(mws...)` | 组合多个中间件 |
| `WrapResolve(resolve, mws...)` | 为单个 resolver 应用中间件 |
| `LoggingMiddleware` | 内置日志中间件 |
| `RecoveryMiddleware` | 内置 panic 恢复中间件 |

## 示例

```go
package main

import (
    "fmt"
    "net/http"

    gql "github.com/graphql-go/graphql"

    "github.com/Tsukikage7/servex/v2/transport/graphql"
)

func main() {
    // 定义 GraphQL Schema
    queryType := gql.NewObject(gql.ObjectConfig{
        Name: "Query",
        Fields: gql.Fields{
            "hello": &gql.Field{
                Type: gql.String,
                Resolve: graphql.WrapResolve(
                    func(p gql.ResolveParams) (any, error) {
                        return "Hello, GraphQL!", nil
                    },
                    graphql.ChainMiddleware(
                        graphql.LoggingMiddleware,
                        graphql.RecoveryMiddleware,
                    ),
                ),
            },
            "user": &gql.Field{
                Type: gql.NewObject(gql.ObjectConfig{
                    Name: "User",
                    Fields: gql.Fields{
                        "id":   &gql.Field{Type: gql.String},
                        "name": &gql.Field{Type: gql.String},
                    },
                }),
                Args: gql.FieldConfigArgument{
                    "id": &gql.ArgumentConfig{Type: gql.String},
                },
                Resolve: func(p gql.ResolveParams) (any, error) {
                    id, _ := p.Args["id"].(string)
                    return map[string]any{"id": id, "name": "Alice"}, nil
                },
            },
        },
    })

    schema, _ := gql.NewSchema(gql.SchemaConfig{Query: queryType})

    srv := graphql.New(graphql.Config{
        Schema:     schema,
        Endpoint:   "/graphql",
        Playground: true,
    })

    http.Handle("/graphql", srv.Handler())
    fmt.Println("GraphQL 服务器启动于 :8080/graphql")
    http.ListenAndServe(":8080", nil)
}
```
