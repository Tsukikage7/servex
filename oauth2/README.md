# oauth2

`github.com/Tsukikage7/servex/v2/oauth2` -- OAuth2 第三方登录 Client 端。

## 概述

oauth2 包提供统一的 OAuth2 第三方登录客户端抽象，内置 GitHub、Google、微信三个 Provider 实现，配合 StateStore 防御 CSRF 攻击，覆盖完整的授权码流程：生成授权 URL -> 用户授权 -> code 换取 token -> 获取用户信息。

## 功能特性

- 统一接口：Provider 抽象适配多个第三方平台
- 内置三大 Provider：GitHub、Google、微信
- CSRF 防御：StateStore 管理 state 参数，一次性消费
- Token 管理：支持 access_token、refresh_token 和过期检测
- StateStore 后端：Memory（开发测试）和 Redis（生产环境）

## 核心类型

### Token

| 字段 | 类型 | 说明 |
|------|------|------|
| `AccessToken` | `string` | 访问令牌 |
| `TokenType` | `string` | 令牌类型（如 Bearer） |
| `RefreshToken` | `string` | 刷新令牌 |
| `ExpiresAt` | `time.Time` | 过期时间 |
| `Scopes` | `[]string` | 授权范围 |
| `Raw` | `map[string]any` | 原始响应 |

| 方法 | 说明 |
|------|------|
| `IsExpired() bool` | 检查 token 是否已过期 |

### UserInfo

| 字段 | 类型 | 说明 |
|------|------|------|
| `ProviderID` | `string` | 第三方平台用户 ID |
| `Provider` | `string` | 平台名称（github/google/wechat） |
| `Name` | `string` | 用户名 |
| `Email` | `string` | 邮箱 |
| `AvatarURL` | `string` | 头像 URL |
| `Extra` | `map[string]any` | 原始用户信息 |

## 核心接口

### Provider

| 方法 | 说明 |
|------|------|
| `AuthURL(state string, opts ...AuthURLOption) string` | 生成第三方授权 URL |
| `Exchange(ctx, code) (*Token, error)` | 用授权码换取 Token |
| `Refresh(ctx, refreshToken) (*Token, error)` | 刷新 Token |
| `UserInfo(ctx, token) (*UserInfo, error)` | 获取用户信息 |

### StateStore

| 方法 | 说明 |
|------|------|
| `Generate(ctx) (string, error)` | 生成并存储 state |
| `Validate(ctx, state) (bool, error)` | 验证并消费 state（一次性） |

### AuthURLOption

| 选项 | 说明 |
|------|------|
| `WithExtraScopes(scopes ...string)` | 追加额外的 scope |
| `WithPrompt(string)` | 设置 prompt 参数（如 "consent"、"login"） |

## Provider 列表

### GitHub (`oauth2/github`)

| 构造函数 | `NewProvider(opts ...Option) *Provider` |
|----------|----------------------------------------|

| 选项 | 说明 |
|------|------|
| `WithClientID(string)` | 设置 Client ID |
| `WithClientSecret(string)` | 设置 Client Secret |
| `WithRedirectURL(string)` | 设置回调 URL |
| `WithScopes(...string)` | 设置 scope（如 "user", "repo"） |
| `WithHTTPClient(*http.Client)` | 自定义 HTTP 客户端 |

> 注：GitHub OAuth2 不支持 Refresh Token。

### Google (`oauth2/google`)

| 构造函数 | `NewProvider(opts ...Option) *Provider` |
|----------|----------------------------------------|

| 选项 | 说明 |
|------|------|
| `WithClientID(string)` | 设置 Client ID |
| `WithClientSecret(string)` | 设置 Client Secret |
| `WithRedirectURL(string)` | 设置回调 URL |
| `WithScopes(...string)` | 设置 scope（默认 openid, profile, email） |
| `WithHTTPClient(*http.Client)` | 自定义 HTTP 客户端 |

### 微信 (`oauth2/wechat`)

| 构造函数 | `NewProvider(opts ...Option) *Provider` |
|----------|----------------------------------------|

| 选项 | 说明 |
|------|------|
| `WithAppID(string)` | 设置微信 AppID |
| `WithAppSecret(string)` | 设置微信 AppSecret |
| `WithHTTPClient(*http.Client)` | 自定义 HTTP 客户端 |

## StateStore 实现

| 后端 | 包路径 | 构造函数 | 说明 |
|------|--------|----------|------|
| Memory | `oauth2/state` | `NewMemoryStore()` | 内存实现，TTL 10 分钟，适合开发测试 |
| Redis | `oauth2/state` | `NewRedisStore(c cache.Cache, opts ...RedisOption)` | 接受 `cache.Cache`，生产环境推荐 |

**RedisStore 选项**

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithPrefix(string)` | `"oauth2:state:"` | Redis key 前缀 |
| `WithTTL(time.Duration)` | `10m` | state 过期时间 |

## 使用示例

```go
import (
    "github.com/Tsukikage7/servex/v2/oauth2"
    "github.com/Tsukikage7/servex/v2/oauth2/github"
    "github.com/Tsukikage7/servex/v2/oauth2/state"
    "github.com/Tsukikage7/servex/v2/storage/cache"
)

// 初始化
provider := github.NewProvider(
    github.WithClientID("your-client-id"),
    github.WithClientSecret("your-secret"),
    github.WithRedirectURL("https://example.com/callback"),
    github.WithScopes("user", "repo"),
)

// StateStore（开发环境）
stateStore := state.NewMemoryStore()

// StateStore（生产环境）：接受 cache.Cache
c := cache.MustNewCache(&cache.Config{Type: cache.TypeRedis, Addr: "localhost:6379"}, log)
stateStore, _ := state.NewRedisStore(c,
    state.WithPrefix("oauth2:state:"),
    state.WithTTL(10 * time.Minute),
)

// 1. 生成授权 URL，重定向用户
stateVal, _ := stateStore.Generate(ctx)
authURL := provider.AuthURL(stateVal, oauth2.WithExtraScopes("read:org"))
// -> 重定向到 authURL

// 2. 回调处理：验证 state + 换取 token + 获取用户信息
func callbackHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 验证 state
    valid, _ := stateStore.Validate(ctx, r.URL.Query().Get("state"))
    if !valid {
        http.Error(w, "invalid state", 403)
        return
    }

    // code 换 token
    token, _ := provider.Exchange(ctx, r.URL.Query().Get("code"))

    // 获取用户信息
    user, _ := provider.UserInfo(ctx, token)
    fmt.Printf("登录用户: %s (%s)\n", user.Name, user.Email)
}
```
