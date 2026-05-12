# auth/casbin

可选 Casbin 授权适配器。`auth` 核心只依赖 `auth.Authorizer` 抽象，本包负责把 Casbin 兼容的 `Enforce(...)` 调用接入 HTTP/gRPC/Endpoint 中间件。

## 快速开始

```go
import (
    casbinlib "github.com/casbin/casbin/v2"

    authcasbin "github.com/Tsukikage7/servex/v2/auth/casbin"
)

enforcer, err := casbinlib.NewEnforcer("model.conf", "policy.csv")
if err != nil {
    return err
}

authorizer := authcasbin.NewAuthorizer(enforcer)
handler = auth.HTTPMiddleware(authenticator, auth.WithAuthorizer(authorizer))(handler)
```

默认映射为：

```go
enforcer.Enforce(principal.ID, target.Resource, target.Action)
```

## 自定义请求

Casbin domain model 或 ABAC model 通常需要不同参数。用 `WithRequestBuilder` 显式声明，不在 `auth` 核心里预设策略模型。

```go
authorizer := authcasbin.NewAuthorizer(enforcer,
    authcasbin.WithRequestBuilder(func(ctx context.Context, p *auth.Principal, target auth.Target) []interface{} {
        tenantID, _ := p.GetMetadata("tenant_id")
        return []interface{}{p.ID, tenantID, target.Resource, target.Action}
    }),
)
```

## 设计边界

- `auth/casbin` 不替代 `auth/rbac`。
- `auth/rbac` 和 `auth/casbin` 都是显式选配。
- 核心 `auth` 包不默认选择任何授权实现。
