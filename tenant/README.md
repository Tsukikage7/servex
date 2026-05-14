# tenant

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/tenant"
```

## 简介

`tenant` 提供多租户支持的核心抽象层。通过 `Tenant` 接口和 context 传播实现租户隔离，提供 HTTP 中间件、gRPC 拦截器、Endpoint 中间件、GORM 作用域、Redis Key 前缀工具，以及限流键函数等多种集成能力。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Tenant` | 租户接口（`ID() string`） |
| `Resolver` | 租户解析器接口（`Resolve(ctx) (Tenant, error)`） |
| `TokenExtractor` | Token 提取器接口 |
| `BearerTokenExtractor` | 从 Authorization Bearer 头提取 |
| `HeaderTokenExtractor(header)` | 从指定请求头提取 |
| `QueryTokenExtractor(param)` | 从 Query 参数提取 |
| `MetadataTokenExtractor(key)` | 从 gRPC metadata 提取 |
| `WithTenant(ctx, tenant)` | 将租户注入 context |
| `FromContext(ctx)` | 从 context 读取租户（可为 nil） |
| `MustFromContext(ctx)` | 从 context 读取租户（不存在则 panic） |
| `ID(ctx)` | 从 context 读取租户 ID |
| `HTTPMiddleware(resolver)` | HTTP 租户注入中间件 |
| `UnaryServerInterceptor(resolver)` | gRPC 一元租户注入拦截器 |
| `Middleware(resolver)` | Endpoint 中间件 |
| `PrefixKey(ctx, key)` | 为 Redis Key 添加租户前缀 |
| `TenantHTTPKeyFunc` | 限流用 HTTP 键函数 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/Tsukikage7/servex/v2/tenant"
)

// SimpleTenant 简单租户实现
type SimpleTenant struct{ id string }

func (t *SimpleTenant) ID() string { return t.id }

// HeaderResolver 从 X-Tenant-ID 头解析租户
type HeaderResolver struct{}

func (r *HeaderResolver) Resolve(ctx context.Context) (tenant.Tenant, error) {
    // 实际应从 ctx 中读取 HTTP 请求头，这里演示逻辑
    return &SimpleTenant{id: "tenant-abc"}, nil
}

func main() {
    resolver := &HeaderResolver{}

    mux := http.NewServeMux()
    mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
        t := tenant.FromContext(r.Context())
        if t == nil {
            http.Error(w, "未找到租户", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, `{"tenant_id":"%s","data":[]}`, t.ID())
    })

    handler := tenant.HTTPMiddleware(resolver)(mux)
    http.ListenAndServe(":8080", handler)
}
```
