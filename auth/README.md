# Auth

统一的认证授权框架，提供 HTTP/gRPC/Endpoint 中间件。

## 结构

```
auth/
├── auth.go        # 核心类型 (Principal, Credentials, 接口定义)
├── errors.go      # 错误定义
├── context.go     # Context 操作
├── authorizer.go  # AuthorizerFunc 与 MethodAuthorizer
├── policy.go      # 方法级声明式授权策略
├── options.go     # 中间件配置选项
├── middleware.go  # Endpoint 中间件
├── http.go        # HTTP 中间件
├── grpc.go        # gRPC 拦截器
├── jwt/           # JWT 认证（可独立使用）
├── rbac/          # 可选 RBAC 授权适配
└── casbin/        # 可选 Casbin 授权适配
```

## 配置选项

```go
auth.HTTPMiddleware(authenticator,
    // 设置日志
    auth.WithLogger(log),

    // 设置授权器（由 rbac/casbin 或业务侧实现）
    auth.WithAuthorizer(myAuthorizer),

    // 设置方法级策略提供者（gateway.WithAuth 会自动注入 proto 策略）
    auth.WithPolicyProvider(policyProvider),

    // 跳过某些路径
    auth.WithSkipper(auth.HTTPSkipPaths("/health", "/ready")),

    // 自定义凭据提取
    auth.WithCredentialsExtractor(auth.BearerExtractor),

    // 自定义错误处理
    auth.WithErrorHandler(func(ctx context.Context, err error) error {
        return customError(err)
    }),
)
```

## 授权器

```go
authorizer := auth.AuthorizerFunc(func(ctx context.Context, principal *auth.Principal, target auth.Target) error {
    if principal == nil {
        return auth.ErrUnauthenticated
    }
    return nil
})

endpoint = auth.Middleware(authenticator, auth.WithAuthorizer(authorizer))(endpoint)
```

`auth` 核心只定义授权抽象，不默认选择 RBAC 或 Casbin。需要角色权限模型时使用 `auth/rbac`，需要 Casbin policy model 时使用 `auth/casbin`。

## 声明式权限策略

`WithPolicyProvider` 用于把 transport 方法映射到认证授权策略。业务侧一般不需要手写；
通过 `transport/gateway.WithAuth(...)` 启用 Gateway 认证时会自动读取 protobuf option。

```go
interceptor := auth.UnaryServerInterceptor(authenticator,
    auth.WithAuthorizer(authorizer),
    auth.WithPolicyProvider(auth.MethodPolicyMap{
        "/api.order.v1.OrderService/Create": {
            FullMethod:  "/api.order.v1.OrderService/Create",
            Permissions: []string{"orders:create"},
        },
    }),
)
```

当 `Permissions` 非空时，必须显式配置 `Authorizer`。默认权限列表是 OR 语义；
`AllPermissions: true` 时切换为 AND 语义。权限字符串默认按 `resource:action` 解析，
例如 `orders:create` 会传入 `Target{Resource: "orders", Action: "create"}`。

## 上下文操作

```go
// 获取身份主体
principal, ok := auth.FromContext(ctx)

// 获取身份主体（不存在则 panic）
principal := auth.MustFromContext(ctx)

// 获取用户 ID
id, ok := auth.GetPrincipalID(ctx)
```

## 错误处理

```go
var (
    auth.ErrUnauthenticated    // 未认证
    auth.ErrForbidden          // 无权限
    auth.ErrInvalidCredentials // 无效凭据
    auth.ErrCredentialsExpired // 凭据已过期
    auth.ErrCredentialsNotFound // 凭据未找到
)

// 错误检查
if auth.IsUnauthenticated(err) { ... }
if auth.IsForbidden(err) { ... }
```

## JWT 子包

JWT 子包可以独立使用，详见 [jwt/README.md](jwt/README.md)。

```go
// 直接使用 JWT（不通过 auth 包）
j := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
handler = jwt.HTTPMiddleware(j)(handler)
```
