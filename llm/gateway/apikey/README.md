# llm/gateway/apikey

`github.com/Tsukikage7/servex/v2/llm/gateway/apikey` — API Key 管理，支持密钥的创建、验证、撤销、配额管理及 HTTP 中间件鉴权。

## 核心类型

- `Key` — API 密钥模型，包含 ID、Name、HashedKey、Prefix、OwnerID、Permissions、RateLimit、QuotaLimit、QuotaUsed、ExpiresAt、Enabled 等字段
- `Manager` — 管理器接口，方法包括 Create、Validate、Revoke、List、UpdateQuota
- `Store` — 存储接口，方法包括 Save、GetByHash、GetByID、Update、List
- `RateLimiter` — 限流接口，方法为 `Allow(ctx, key, limit) (bool, error)`
- `NewManager(store, opts...)` — 创建 Manager，密钥前缀默认 "sk-"
- `HTTPMiddleware(mgr)` — 返回 HTTP 鉴权中间件，从 `Authorization: Bearer` 或 `X-API-Key` 头部提取密钥
- `FromContext(ctx)` — 从 context 中获取已验证的 Key
- `NewContext(ctx, key)` — 将 Key 注入 context

## 使用示例

```go
import "github.com/Tsukikage7/servex/v2/llm/gateway/apikey"

mgr, _ := apikey.NewManager(myStore,
    apikey.WithKeyPrefix("sk-"),
)

// 创建 Key
rawKey, key, _ := mgr.Create(ctx,
    apikey.WithName("我的应用"),
    apikey.WithOwnerID("user-123"),
    apikey.WithQuotaLimit(1_000_000),
    apikey.WithRateLimit(60),
)
fmt.Println("原始密钥（仅显示一次）:", rawKey)

// 验证 Key
validKey, err := mgr.Validate(ctx, rawKey)
if err != nil {
    // ErrKeyExpired / ErrQuotaExceeded / ErrRateLimited ...
}

// HTTP 中间件
mux := http.NewServeMux()
mux.Handle("/v1/", apikey.HTTPMiddleware(mgr)(myHandler))
_ = key
_ = validKey
```
