# servex 多租户

## Tenant 接口与 Context

```go
import "github.com/Tsukikage7/servex/v2/tenant"

// 应用层实现 Tenant 接口
type MyTenant struct {
    ID      string
    Name    string
    Enabled bool
}
func (t *MyTenant) TenantID() string    { return t.ID }
func (t *MyTenant) TenantEnabled() bool { return t.Enabled }

// 存入 / 取出 context
ctx = tenant.WithTenant(ctx, &MyTenant{ID: "t1", Enabled: true})
t, ok := tenant.FromContext(ctx)     // 安全提取
t = tenant.MustFromContext(ctx)       // 不存在则 panic
id := tenant.ID(ctx)                 // 直接获取 ID，无租户返回 ""
```

## Resolver -- 租户解析器

```go
import "github.com/Tsukikage7/servex/v2/tenant"

// 实现 Resolver 接口
type DBResolver struct{ db *gorm.DB }

func (r *DBResolver) Resolve(ctx context.Context, token string) (tenant.Tenant, error) {
    var t MyTenant
    if err := r.db.Where("api_key = ?", token).First(&t).Error; err != nil {
        return nil, err
    }
    return &t, nil
}
```

## TokenExtractor -- 令牌提取

```go
// 从 Authorization: Bearer <token> 提取
tenant.BearerTokenExtractor()

// 从自定义 HTTP 头提取
tenant.HeaderTokenExtractor("X-Tenant-ID")

// 从 URL 查询参数提取
tenant.QueryTokenExtractor("tenant_id")

// 从 gRPC metadata 提取
tenant.MetadataTokenExtractor("x-tenant-id")

// 从 auth.Principal 桥接（auth → tenant）
tenant.PrincipalTokenExtractor()
```

## HTTP 中间件

```go
import "github.com/Tsukikage7/servex/v2/tenant"

resolver := &DBResolver{db: db}

handler = tenant.HTTPMiddleware(resolver,
    tenant.WithTokenExtractor(tenant.HeaderTokenExtractor("X-Tenant-ID")),
    tenant.WithLogger(log),
    tenant.WithSkipper(tenant.HTTPSkipPaths("/health", "/api/public/*")),
)(handler)
```

**中间件流程：** skipper 检查 → 提取 token → resolve 租户 → 检查 enabled → WithTenant(ctx) → next

**跳过路径：** `HTTPSkipPaths` 支持精确匹配和通配前缀（以 `*` 结尾）

## Endpoint 中间件

```go
import "github.com/Tsukikage7/servex/v2/tenant"

// 用于 endpoint 层（transport 无关）
ep = tenant.Middleware(resolver,
    tenant.WithTokenExtractor(extractorFn),
)(ep)
```

## SQL 作用域（通用）

```go
import "github.com/Tsukikage7/servex/v2/tenant"

// 获取 WHERE 子句
clause, args := tenant.WhereClause(ctx)
// → ("tenant_id = ?", ["t1"])

// 自定义列名
clause, args = tenant.WhereClause(ctx, "t.tenant_id")
// → ("t.tenant_id = ?", ["t1"])
```

## GORM 集成

```go
import tenantgorm "github.com/Tsukikage7/servex/v2/tenant/gorm"

// 查询作用域 — 自动按 tenant_id 过滤
db.Scopes(tenantgorm.Scope(ctx)).Find(&results)
// SQL: SELECT * FROM ... WHERE tenant_id = 't1'

// 自定义列名
db.Scopes(tenantgorm.Scope(ctx, "orders.tenant_id")).Find(&results)

// 自动注入 — Create/Update 时自动设置 tenant_id
if err := tenantgorm.AutoInject(db); err != nil {
    log.Fatal(err)
}
// 之后所有 db.Create(&record) 会自动从 ctx 获取 tenant_id 并设置到记录中
```

## 完整示例

```go
// 初始化
resolver := &DBResolver{db: db}
tenantgorm.AutoInject(db) // 注册 GORM 回调

// HTTP 路由
mux.Handle("/api/", tenant.HTTPMiddleware(resolver,
    tenant.WithTokenExtractor(tenant.HeaderTokenExtractor("X-Tenant-ID")),
    tenant.WithSkipper(tenant.HTTPSkipPaths("/api/auth/*")),
)(apiHandler))

// 业务层查询自动隔离
func ListOrders(ctx context.Context) ([]Order, error) {
    var orders []Order
    err := db.WithContext(ctx).Scopes(tenantgorm.Scope(ctx)).Find(&orders).Error
    return orders, err
}
```
