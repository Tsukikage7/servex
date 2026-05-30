# servex 认证

## auth/jwt — JWT 签发与验证

```go
// 创建 JWT 服务 — HMAC 对称签名（缺少 WithLogger 会 panic）
jwtSrv := jwt.MustNew(
    jwt.WithLogger(log),
    jwt.WithSecretKey("your-secret-key"),
    jwt.WithIssuer("my-service"),
    jwt.WithAccessDuration(2 * time.Hour),
    jwt.WithRefreshDuration(7 * 24 * time.Hour),
)

// RSA 非对称签名（RS256）
rsaKey, err := jwt.LoadRSAPrivateKey("/etc/keys/private.pem")
if err != nil { ... }
jwtSrv := jwt.MustNew(
    jwt.WithLogger(log),
    jwt.WithRSAKeys(rsaKey, &rsaKey.PublicKey),
    jwt.WithIssuer("my-service"),
)

// ECDSA 非对称签名（ES256）
jwtSrv := jwt.MustNew(
    jwt.WithLogger(log),
    jwt.WithECDSAKeys(ecdsaPrivateKey, &ecdsaPrivateKey.PublicKey),
)

// EdDSA 签名（Ed25519）
jwtSrv := jwt.MustNew(
    jwt.WithLogger(log),
    jwt.WithEdDSAKeys(ed25519PrivateKey, ed25519PublicKey),
)

// 签发令牌
claims := &jwt.StandardClaims{
    RegisteredClaims: gojwt.RegisteredClaims{
        Subject: "user-123",
    },
}
tokenStr, err := jwtSrv.Generate(claims)

// 验证令牌
parsed, err := jwtSrv.Validate(tokenStr)
sub, _ := parsed.GetSubject()
```

完整示例：`docs/superpowers/examples/jwt/main.go`

**纯 HTTP 集成：**

```go
// NewAuthenticator 将 JWT 服务包装为 auth.Authenticator 接口
authenticator := jwt.NewAuthenticator(jwtSrv)

router := httpserver.NewRouter()
router.POST("/login", httpserver.Handle(loginHandler)) // 公开路由不挂认证中间件

api := router.Group("/api/v1", auth.HTTPMiddleware(authenticator))
api.GET("/profile", httpserver.Handle(profileHandler))

srv := httpserver.New(router,
    httpserver.WithLogger(log),
)
```

**关键类型：**
- `jwt.StandardClaims` — 标准 claims 结构（嵌入 `gojwt.RegisteredClaims`）
- `auth.Principal` — 认证后的用户信息，含 `ID`（不是 `UserID`）

**统一错误码（v2.0.6+）：** auth 包的所有错误已统一为 servex errors 格式，包含 HTTP/gRPC 映射：
- `auth.ErrUnauthenticated(20001)` — 未认证（401 / Unauthenticated）
- `auth.ErrForbidden(20002)` — 无权限（403 / PermissionDenied）
- `jwt.ErrTokenInvalid(20101)` — 令牌无效（401 / Unauthenticated）
- `jwt.ErrTokenRevoked(20102)` — 令牌已撤销（401 / Unauthenticated）
- `rbac.ErrPermissionDenied(20303)` — RBAC 权限被拒绝（403 / PermissionDenied）

**签名算法选项：**
- `WithSecretKey(key)` — HMAC 对称签名（HS256，默认）
- `WithRSAKeys(privateKey, publicKey)` — RSA 非对称签名（RS256）
- `WithECDSAKeys(privateKey, publicKey)` — ECDSA 非对称签名（ES256）
- `WithEdDSAKeys(privateKey, publicKey)` — EdDSA 签名（Ed25519）
- `LoadRSAPrivateKey(path)` — 从 PEM 文件加载 RSA 私钥

## auth/apikey — API Key 验证

```go
// StaticValidator：硬编码 key 列表（适合内部服务、测试）
validator := apikey.StaticValidator(map[string]string{
    "key-abc": "service-a",
    "key-xyz": "service-b",
})

// CacheValidator：带缓存的动态验证（适合从数据库查询 key）
validator := apikey.CacheValidator(
    func(ctx context.Context, key string) (string, error) {
        // 返回 subject（用户ID/服务名），查不到返回 error
        return db.LookupAPIKey(ctx, key)
    },
    5*time.Minute, // 缓存 TTL
)

// 包装为 Authenticator 接口
authenticator := apikey.New(validator)

// 纯 HTTP 集成：公开路由不挂认证中间件，受保护路由显式分组
router := httpserver.NewRouter()
api := router.Group("/api", auth.HTTPMiddleware(authenticator))
api.GET("/internal/status", httpserver.Handle(statusHandler))
```

**关键选项：**
- `apikey.New(validator)` — 构造 `*Authenticator`，不是 `NewAuthenticator`
- `StaticValidator` — 返回 `Validator` 函数类型
- `CacheValidator(lookupFn, ttl)` — 带内存缓存的动态验证

## 授权模型

`auth` 核心只定义 `auth.Authorizer` 抽象，不默认选择 RBAC、Casbin 或其它策略引擎。需要角色权限模型时显式使用 `auth/rbac`；需要 Casbin policy model 时显式使用 `auth/casbin`。

```go
authorizer := auth.AuthorizerFunc(func(ctx context.Context, principal *auth.Principal, target auth.Target) error {
    if principal == nil {
        return auth.ErrUnauthenticated
    }
    return nil
})

handler = auth.HTTPMiddleware(authenticator, auth.WithAuthorizer(authorizer))(handler)
```

## auth/proto — 声明式认证与授权策略

Gateway / gRPC 标准用法是在 `.proto` 中声明公开接口和权限要求，代码侧只启用认证器和可选授权器，不再维护手动公开方法列表。

```protobuf
import "auth/proto/auth.proto";

service UserService {
  option (microservice.kit.auth.service) = {
    default_permissions: ["user:read"]
  };

  rpc Login(LoginRequest) returns (LoginResponse) {
    option (microservice.kit.auth.method) = {
      public: true
    };
  }

  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse) {
    option (microservice.kit.auth.method) = {
      permissions: ["user:delete", "admin"]
      all_permissions: true
    };
  }
}
```

```go
srv := gateway.New(
    gateway.WithSecurity(gateway.SecurityConfig{
        Authenticator: authenticator,
        AuthOptions:   []auth.Option{auth.WithAuthorizer(authorizer)},
    }),
)
```

规则：
- `public: true`：跳过认证，适合登录、注册、健康检查等公共接口。
- `permissions`：认证通过后交给显式配置的 `auth.Authorizer` 校验。
- `all_permissions: true`：要求满足全部权限；默认是任一权限即可。
- 没有 proto option：不公开，也不附加权限要求，按普通认证接口处理。
- 声明了权限但没有配置 `auth.Authorizer`：请求会被拒绝，避免策略静默失效。

## auth/rbac — 可选 RBAC 授权适配

```go
// 创建管理器（内存存储适合测试，GORM 存储适合生产）
store := rbac.NewMemoryStore()
// store := rbac.NewGORMStore(gormDB); store.AutoMigrate(ctx)

mgr := rbac.NewManager(store,
    rbac.WithSuperAdmin("superadmin"),
)

// 创建角色（权限格式：resource:action，支持通配符 *）
_ = mgr.CreateRole(ctx, &rbac.Role{
    Name:        "editor",
    Permissions: []string{"articles:read", "articles:write"},
})

// 角色继承（admin 继承 editor 的所有权限）
_ = mgr.CreateRole(ctx, &rbac.Role{
    Name:        "admin",
    Permissions: []string{"users:*"},
    ParentID:    "editor",
})

// 分配 / 撤销角色
_ = mgr.AssignRole(ctx, "user-1", "editor")
_ = mgr.RevokeRole(ctx, "user-1", "editor")

// 权限检查
ok, _ := mgr.HasPermission(ctx, "user-1", "articles", "read")

// 获取用户所有权限
perms, _ := mgr.GetUserPermissions(ctx, "user-1")
```

**auth 中间件集成：**

```go
authorizer := rbac.NewAuthorizer(mgr)
handler = auth.HTTPMiddleware(
    authenticator,
    auth.WithAuthorizer(authorizer),
    auth.WithTarget(auth.Target{Resource: "articles", Action: "write"}),
)(handler)
```

**缓存集成：**

```go
mgr := rbac.NewManager(store,
    rbac.WithCache(func(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
        return cacheClient.GetOrSet(ctx, key, ttl, fn)
    }),
)
```

**关键类型：**
- `rbac.RBAC` — 权限管理器接口
- `rbac.Role` — 角色（ID/Name/Permissions/ParentID/Description）
- `rbac.Store` — 存储接口（`NewMemoryStore` / `NewGORMStore`）
- `rbac.NewAuthorizer(mgr)` — 转为 `auth.Authorizer`
- `rbac.ParsePermission("resource:action")` — 解析权限字符串

## auth/casbin — 可选 Casbin 授权适配

```go
import authcasbin "github.com/Tsukikage7/servex/v2/auth/casbin"

authorizer := authcasbin.NewAuthorizer(enforcer)
handler = auth.HTTPMiddleware(authenticator, auth.WithAuthorizer(authorizer))(handler)
```

默认映射为：

```go
enforcer.Enforce(principal.ID, target.Resource, target.Action)
```

domain model 或 ABAC model 用 `WithRequestBuilder` 显式声明：

```go
authorizer := authcasbin.NewAuthorizer(enforcer,
    authcasbin.WithRequestBuilder(func(ctx context.Context, p *auth.Principal, target auth.Target) []interface{} {
        tenantID, _ := p.GetMetadata("tenant_id")
        return []interface{}{p.ID, tenantID, target.Resource, target.Action}
    }),
)
```
