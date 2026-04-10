# servex 认证

## auth/jwt — JWT 签发与验证

```go
// 创建 JWT 服务 — HMAC 对称签名（缺少 WithLogger 会 panic）
jwtSrv := jwt.NewJWT(
    jwt.WithLogger(log),
    jwt.WithSecretKey("your-secret-key"),
    jwt.WithIssuer("my-service"),
    jwt.WithAccessDuration(2 * time.Hour),
    jwt.WithRefreshDuration(7 * 24 * time.Hour),
)

// RSA 非对称签名（RS256）
rsaKey, err := jwt.LoadRSAPrivateKey("/etc/keys/private.pem")
if err != nil { ... }
jwtSrv := jwt.NewJWT(
    jwt.WithLogger(log),
    jwt.WithRSAKeys(rsaKey, &rsaKey.PublicKey),
    jwt.WithIssuer("my-service"),
)

// ECDSA 非对称签名（ES256）
jwtSrv := jwt.NewJWT(
    jwt.WithLogger(log),
    jwt.WithECDSAKeys(ecdsaPrivateKey, &ecdsaPrivateKey.PublicKey),
)

// EdDSA 签名（Ed25519）
jwtSrv := jwt.NewJWT(
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

**与 httpserver 集成：**

```go
// NewAuthenticator 将 JWT 服务包装为 auth.Authenticator 接口
authenticator := jwt.NewAuthenticator(jwtSrv)

srv := httpserver.New(mux,
    httpserver.WithLogger(log),
    httpserver.WithAuth(authenticator, "/api/login", "/healthz"), // 白名单路径无需认证
)
```

**关键类型：**
- `jwt.StandardClaims` — 标准 claims 结构（嵌入 `gojwt.RegisteredClaims`）
- `auth.Principal` — 认证后的用户信息，含 `ID`（不是 `UserID`）

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

// 集成到 httpserver（从 X-API-Key header 读取）
srv := httpserver.New(mux,
    httpserver.WithAuth(authenticator, "/healthz"),
)
```

**关键选项：**
- `apikey.New(validator)` — 构造 `*Authenticator`，不是 `NewAuthenticator`
- `StaticValidator` — 返回 `Validator` 函数类型
- `CacheValidator(lookupFn, ttl)` — 带内存缓存的动态验证

## auth/rbac — 基于角色的访问控制

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

**HTTP 中间件：**

```go
// 从 auth.FromContext 取 userID，检查 resource + action
mux.Handle("/articles", rbac.HTTPMiddleware(mgr, "articles", "write")(handler))
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
- `rbac.HTTPMiddleware(mgr, resource, action)` — HTTP 鉴权中间件
- `rbac.ParsePermission("resource:action")` — 解析权限字符串
