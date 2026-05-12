# auth/rbac

可选的基于角色访问控制（Role-Based Access Control）适配，支持角色继承、权限通配符、超级管理员和可插拔存储后端。

## 特性

- 角色管理：创建、删除、列表
- 用户角色分配与撤销
- 权限格式：`resource:action`（如 `articles:read`），支持通配符 `*`
- 角色继承：通过 `ParentID` 字段形成角色层级
- 超级管理员角色（拥有所有权限）
- 可选缓存层（注入任意缓存函数）
- HTTP 中间件（与 `auth.Principal` 集成）
- 两种存储实现：内存（测试用）和 GORM（生产用）

## 快速开始

```go
import "github.com/Tsukikage7/servex/v2/auth/rbac"

// 内存存储（测试）
store := rbac.NewMemoryStore()

// GORM 存储（生产）
// store := rbac.NewGORMStore(gormDB)
// _ = store.AutoMigrate(ctx)

mgr := rbac.NewManager(store,
    rbac.WithSuperAdmin("superadmin"), // 超级管理员角色名
)

// 创建角色
_ = mgr.CreateRole(ctx, &rbac.Role{
    Name:        "editor",
    Description: "内容编辑",
    Permissions: []string{"articles:read", "articles:write"},
})

// 角色继承：admin 继承 editor 的所有权限
_ = mgr.CreateRole(ctx, &rbac.Role{
    Name:        "admin",
    Permissions: []string{"users:*"},
    ParentID:    "editor",
})

// 分配角色
_ = mgr.AssignRole(ctx, "user-1", "editor")

// 权限检查
ok, _ := mgr.HasPermission(ctx, "user-1", "articles", "read") // true
ok, _ = mgr.HasPermission(ctx, "user-1", "users", "delete")   // false
```

## HTTP 中间件

```go
authorizer := rbac.NewAuthorizer(mgr)
handler = auth.HTTPMiddleware(
    authenticator,
    auth.WithAuthorizer(authorizer),
    auth.WithTarget(auth.Target{Resource: "articles", Action: "write"}),
)(handler)
```

## 缓存集成

```go
mgr := rbac.NewManager(store,
    rbac.WithCache(func(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
        return cache.GetOrSet(ctx, key, ttl, fn)
    }),
)
```

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `rbac.RBAC` | 权限管理器接口 |
| `rbac.Role` | 角色（ID/Name/Permissions/ParentID） |
| `rbac.Permission` | 权限（Resource + Action） |
| `rbac.Store` | 存储接口 |
| `NewManager(store, opts...)` | 创建管理器 |
| `NewAuthorizer(mgr)` | 转为 `auth.Authorizer` |
| `NewMemoryStore()` | 内存存储 |
| `NewGORMStore(db)` | GORM 存储 |
| `ParsePermission(s)` | 解析 `"resource:action"` 字符串 |
