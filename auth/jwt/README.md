# JWT

JWT 认证服务，提供令牌生成、验证、刷新和撤销功能。

## 特性

- 生成、验证、刷新令牌
- 可选的缓存集成（用于令牌撤销）
- Endpoint/HTTP 中间件
- gRPC 适配（`auth/jwt/grpcx` 子包）
- 白名单支持
- 自定义 Claims
- Functional Options 模式

## 配置选项

| 选项                  | 默认值       | 说明               |
| --------------------- | ------------ | ------------------ |
| `WithName`            | `JWT`        | 服务名称           |
| `WithSecretKey`       | -            | 签名密钥（必需）   |
| `WithIssuer`          | -            | 签发者             |
| `WithAccessDuration`  | `2h`         | 访问令牌有效期     |
| `WithRefreshDuration` | `7d`         | 刷新令牌有效期     |
| `WithRefreshWindow`   | `1h`         | 过期后可刷新窗口   |
| `WithTokenPrefix`     | `Bearer `    | 令牌前缀           |
| `WithCacheKeyPrefix`  | `jwt:token:` | 缓存 key 前缀      |
| `WithCache`           | -            | 缓存实例           |
| `WithLogger`          | -            | 日志记录器（必需） |
| `WithWhitelist`       | -            | 白名单配置         |

## 自定义 Claims

```go
// 定义自定义 Claims
type UserClaims struct {
    jwt.StandardClaims
    UserID   int64  `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
}

// 实现 Claims 接口
func (c *UserClaims) GetSubject() string {
    return fmt.Sprintf("%d", c.UserID)
}

// 使用自定义 Claims
claims := &UserClaims{
    StandardClaims: jwt.StandardClaims{
        RegisteredClaims: jwtv5.RegisteredClaims{
            Subject:   "123",
            ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(2 * time.Hour)),
            IssuedAt:  jwtv5.NewNumericDate(time.Now()),
        },
    },
    UserID:   123,
    Username: "john",
    Role:     "admin",
}

token, err := j.Generate(claims)

// 验证时使用自定义类型
validatedClaims, err := j.ValidateWithClaims(token, &UserClaims{})
userClaims := validatedClaims.(*UserClaims)
```

## Authenticator 与自定义 Claims

`NewAuthenticator` 可直接接入 `auth` / `gateway.WithAuth`。如果令牌中包含业务字段
（如 `roles`、`permissions`、`username`、`tenant_id`），应通过 `WithClaimsFactory`
指定解析类型；认证器会为每次请求创建新的 Claims 实例，避免并发复用。

```go
type UserClaims struct {
    jwt.StandardClaims
    Username string   `json:"username"`
    Roles    []string `json:"roles"`
}

jwtSrv := jwt.NewJWT(
    jwt.WithSecretKey("your-secret-key-at-least-32-bytes"),
    jwt.WithLogger(log),
)

authenticator := jwt.NewAuthenticator(jwtSrv,
    jwt.WithClaimsFactory(func() jwt.Claims {
        return &UserClaims{}
    }),
    jwt.WithClaimsMapper(func(claims gojwt.Claims) (*auth.Principal, error) {
        c, ok := claims.(*UserClaims)
        if !ok {
            return nil, auth.ErrInvalidCredentials
        }
        return &auth.Principal{
            ID:    c.Subject,
            Type:  auth.PrincipalTypeUser,
            Name:  c.Username,
            Roles: c.Roles,
            Metadata: map[string]any{
                "claims": c,
            },
        }, nil
    }),
)

server := gateway.New(
    gateway.WithAuth(authenticator),
)
```

默认 Claims mapper 支持以下通用字段：

- `sub` / `Subject` → `Principal.ID`
- `exp` / `ExpiresAt` → `Principal.ExpiresAt`
- `roles` / `Roles` → `Principal.Roles`
- `role` / `Role` → 单个角色，追加到 `Principal.Roles`
- `permissions` / `Permissions` → `Principal.Permissions`
- `name` / `Name` / `Username` → `Principal.Name`
- `type` / `Type` → `Principal.Type`

## Endpoint 中间件

Endpoint 中间件用于 `transport.Endpoint` 层，参考 [go-kit/kit/auth/jwt](https://github.com/go-kit/kit/tree/master/auth/jwt) 设计模式。

### NewSigner - 签名中间件（客户端）

用于客户端在发起请求前签名令牌：

```go
import (
    "github.com/Tsukikage7/servex/v2/transport"
    "github.com/Tsukikage7/servex/v2/auth/jwt"
)

// 创建签名中间件
signerMiddleware := jwt.NewSigner(j)

// 应用到 endpoint
var clientEndpoint transport.Endpoint = func(ctx context.Context, req any) (any, error) {
    // 令牌已存入上下文，可通过 jwt.TokenFromContext(ctx) 获取
    return callRemoteService(ctx, req)
}
clientEndpoint = signerMiddleware(clientEndpoint)

// 使用时将 Claims 放入上下文
ctx := jwt.ContextWithClaims(context.Background(), claims)
resp, err := clientEndpoint(ctx, request)
```

### NewParser - 解析中间件（服务端）

用于服务端验证传入请求的令牌：

```go
// 创建解析中间件
parserMiddleware := jwt.NewParser(j)

// 应用到 endpoint
var serverEndpoint transport.Endpoint = func(ctx context.Context, req any) (any, error) {
    // 从上下文获取已验证的 Claims
    claims, ok := jwt.ClaimsFromContext(ctx)
    if ok {
        subject := claims.GetSubject()
        log.Printf("用户: %s", subject)
    }
    return process(req)
}
serverEndpoint = parserMiddleware(serverEndpoint)
```

### NewParserWithClaims - 自定义 Claims 解析

```go
// 使用自定义 Claims 类型
parserMiddleware := jwt.NewParserWithClaims(j, func() jwt.Claims {
    return &UserClaims{}
})

serverEndpoint = parserMiddleware(serverEndpoint)

// 在 endpoint 中获取自定义 Claims
claims, ok := jwt.ClaimsFromContext(ctx)
if ok {
    userClaims := claims.(*UserClaims)
    log.Printf("用户ID: %d, 角色: %s", userClaims.UserID, userClaims.Role)
}
```

### 与其他中间件组合

```go
// 服务端推荐顺序
serverEndpoint = transport.Chain(
    metrics.EndpointMiddleware(collector, "my-service", "MyMethod"),
    trace.EndpointMiddleware("my-service", "MyMethod"),
    ratelimit.EndpointMiddleware(limiter),
    jwt.NewParser(j), // JWT 验证
)(serverEndpoint)

// 客户端推荐顺序
clientEndpoint = transport.Chain(
    metrics.EndpointMiddleware(collector, "my-service", "MyMethod"),
    trace.EndpointMiddleware("my-service", "MyMethod"),
    jwt.NewSigner(j), // JWT 签名
    retry.EndpointMiddleware(retryConfig),
)(clientEndpoint)
```

## HTTP 中间件

```go
mux := http.NewServeMux()
mux.HandleFunc("/api/users", usersHandler)

// 使用中间件（与其他中间件风格一致）
var handler http.Handler = mux
handler = jwt.HTTPMiddleware(j)(handler)

// 使用自定义 Claims 类型
handler = jwt.HTTPMiddlewareWithClaims(j, func() jwt.Claims {
    return &UserClaims{}
})(handler)

http.ListenAndServe(":8080", handler)
```

## gRPC 拦截器

```go
// 一元拦截器
srv := grpc.NewServer(
    grpc.UnaryInterceptor(jwtgrpcx.UnaryServerInterceptor(j)),
)

// 流拦截器
srv := grpc.NewServer(
    grpc.StreamInterceptor(jwtgrpcx.StreamServerInterceptor(j)),
)

// 链式使用
srv := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        jwtgrpcx.UnaryServerInterceptor(j),
        otherInterceptor,
    ),
    grpc.ChainStreamInterceptor(
        jwtgrpcx.StreamServerInterceptor(j),
        otherStreamInterceptor,
    ),
)

// 使用自定义 Claims 类型
srv := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        jwtgrpcx.UnaryServerInterceptorWithClaims(j, func() jwt.Claims {
            return &UserClaims{}
        }),
    ),
)
```

## 白名单配置

```go
whitelist := jwt.NewWhitelist().
    AddHTTPPaths("/health", "/ready", "/api/public/").
    AddGRPCMethods("/grpc.health.v1.Health/").
    SetInternalServiceHeader("x-internal-service")

j := jwt.NewJWT(
    jwt.WithSecretKey("secret"),
    jwt.WithLogger(log),
    jwt.WithWhitelist(whitelist),
)
```

## 缓存集成

启用缓存可以实现令牌撤销功能：

```go
j := jwt.NewJWT(
    jwt.WithSecretKey("secret"),
    jwt.WithLogger(log),
    jwt.WithCache(redisCache),
    jwt.WithCacheKeyPrefix("myapp:jwt:"),
)

// 撤销用户所有令牌
j.Revoke(ctx, "user-123")
```

## 从上下文获取信息

```go
// 在中间件验证后的处理函数中
func handler(w http.ResponseWriter, r *http.Request) {
    // 获取 Claims
    claims, ok := jwt.ClaimsFromContext(r.Context())
    if !ok {
        // 未认证
        return
    }

    // 获取 Subject
    subject, ok := jwt.GetSubjectFromContext(r.Context())

    // 获取 Token
    token, ok := jwt.TokenFromContext(r.Context())
}
```

## 令牌刷新

```go
// 刷新令牌
newClaims := &UserClaims{
    StandardClaims: jwt.StandardClaims{
        RegisteredClaims: jwtv5.RegisteredClaims{
            Subject:   "123",
            ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(2 * time.Hour)),
            IssuedAt:  jwtv5.NewNumericDate(time.Now()),
        },
    },
    UserID:   123,
    Username: "john",
    Role:     "admin",
}

newToken, err := j.RefreshWithClaims(oldToken, &UserClaims{}, newClaims)
```

## 完整示例

```go
package main

import (
    "net/http"
    "time"

    jwtv5 "github.com/golang-jwt/jwt/v5"
    "github.com/Tsukikage7/servex/v2/auth/jwt"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

type UserClaims struct {
    jwt.StandardClaims
    UserID   int64  `json:"user_id"`
    Username string `json:"username"`
}

func (c *UserClaims) GetSubject() string {
    return fmt.Sprintf("%d", c.UserID)
}

func main() {
    log := logger.New()

    // 白名单
    whitelist := jwt.NewWhitelist().
        AddHTTPPaths("/health", "/login")

    // JWT 服务
    j := jwt.NewJWT(
        jwt.WithSecretKey("your-secret-key"),
        jwt.WithIssuer("my-service"),
        jwt.WithAccessDuration(2 * time.Hour),
        jwt.WithLogger(log),
        jwt.WithWhitelist(whitelist),
    )

    // 路由
    mux := http.NewServeMux()
    mux.HandleFunc("/health", healthHandler)
    mux.HandleFunc("/login", loginHandler(j))
    mux.HandleFunc("/api/me", meHandler)

    // 启动服务（应用中间件）
    var handler http.Handler = mux
    handler = jwt.HTTPMiddleware(j)(handler)
    http.ListenAndServe(":8080", handler)
}

func loginHandler(j *jwt.JWT) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 验证用户...

        claims := &UserClaims{
            StandardClaims: jwt.StandardClaims{
                RegisteredClaims: jwtv5.RegisteredClaims{
                    Subject:   "123",
                    ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(j.AccessDuration())),
                    IssuedAt:  jwtv5.NewNumericDate(time.Now()),
                    Issuer:    j.Issuer(),
                },
            },
            UserID:   123,
            Username: "john",
        }

        token, err := j.Generate(claims)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.Write([]byte(token))
    }
}

func meHandler(w http.ResponseWriter, r *http.Request) {
    claims, ok := jwt.ClaimsFromContext(r.Context())
    if !ok {
        http.Error(w, "未认证", http.StatusUnauthorized)
        return
    }

    w.Write([]byte("Hello, " + claims.GetSubject()))
}
```

## 错误处理

```go
var (
    ErrTokenInvalid   = errors.New("jwt: 令牌无效或已过期")
    ErrTokenRevoked   = errors.New("jwt: 令牌已撤销")
    ErrTokenEmpty     = errors.New("jwt: 令牌不能为空")
    ErrTokenNotFound  = errors.New("jwt: 未找到认证令牌")
    ErrSigningMethod  = errors.New("jwt: 无效的签名方法")
    ErrClaimsInvalid  = errors.New("jwt: 无效的 Claims")
    ErrRefreshExpired = errors.New("jwt: 令牌已超出刷新窗口")
)
```

**注意**: 如果未设置 `secretKey` 或 `logger`，`New()` 会 panic。
