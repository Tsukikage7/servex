# servex OAuth2 第三方登录

## 核心接口

```go
import "github.com/Tsukikage7/servex/v2/oauth2"

type Provider interface {
    AuthURL(state string, opts ...AuthURLOption) string
    Exchange(ctx context.Context, code string) (*Token, error)
    Refresh(ctx context.Context, refreshToken string) (*Token, error)
    UserInfo(ctx context.Context, token *Token) (*UserInfo, error)
}

type StateStore interface {
    Generate(ctx context.Context) (string, error)
    Validate(ctx context.Context, state string) (bool, error) // 一次性消费
}
```

## 完整登录流程

```go
import (
    "github.com/Tsukikage7/servex/v2/oauth2/github"
    "github.com/Tsukikage7/servex/v2/oauth2/state"
)

gh := github.NewProvider(
    github.WithClientID("xxx"),
    github.WithClientSecret("xxx"),
    github.WithRedirectURL("https://myapp.com/callback"),
    github.WithScopes("user:email", "read:org"),
)

stateStore := state.NewMemoryStore() // 开发用

// 1. 授权页面
func loginHandler(w http.ResponseWriter, r *http.Request) {
    s, _ := stateStore.Generate(r.Context())
    http.Redirect(w, r, gh.AuthURL(s), http.StatusFound)
}

// 2. 回调处理
func callbackHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    ok, _ := stateStore.Validate(ctx, r.URL.Query().Get("state"))
    if !ok { /* CSRF 攻击 */ }

    token, _ := gh.Exchange(ctx, r.URL.Query().Get("code"))
    user, _ := gh.UserInfo(ctx, token)

    // 映射为内部 Principal（应用层做，oauth2 包不导入 auth）
    principal := &auth.Principal{
        ID: user.ProviderID, Name: user.Name, Type: "user",
        Metadata: map[string]any{"provider": user.Provider},
    }
}
```

## Provider 列表

| Provider | 包路径 | 选项 | 特点 |
|----------|--------|------|------|
| GitHub | `oauth2/github` | `WithClientID`, `WithClientSecret`, `WithRedirectURL`, `WithScopes` | 不支持 refresh |
| Google | `oauth2/google` | 同上 | 支持 refresh，默认 scope: openid/profile/email |
| 微信 | `oauth2/wechat` | `WithAppID`, `WithAppSecret` | 扫码登录，unionid 作为 ProviderID |

## StateStore

```go
// 内存（开发/测试，10 分钟 TTL，一次性消费）
store := state.NewMemoryStore()

// Redis（生产）：接受 cache.Cache
import "github.com/Tsukikage7/servex/v2/storage/cache"
c := cache.MustNewCache(&cache.Config{Type: cache.TypeRedis, Addr: "localhost:6379"}, log)
store, _ := state.NewRedisStore(c,
    state.WithPrefix("oauth2:state:"),
    state.WithTTL(10 * time.Minute),
)
```

## AuthURL 选项

```go
url := gh.AuthURL(s,
    oauth2.WithExtraScopes("repo"),     // 追加额外 scope
    oauth2.WithPrompt("consent"),        // 强制重新授权（Google）
)
```
