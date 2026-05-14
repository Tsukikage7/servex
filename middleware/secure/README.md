# middleware/secure

HTTP 安全头中间件，自动为每个响应注入常见安全相关 header，防御点击劫持、MIME 嗅探、XSS 等攻击。

## 功能

- `X-Frame-Options` — 防止点击劫持（默认 `DENY`）
- `X-Content-Type-Options: nosniff` — 防止 MIME 类型嗅探（默认启用）
- `X-XSS-Protection` — 旧版浏览器 XSS 过滤（默认 `1; mode=block`）
- `Strict-Transport-Security` — HSTS，强制 HTTPS（默认 max-age=31536000，含子域名）
- `Content-Security-Policy` — CSP，限制资源加载来源（默认不设置，需手动指定）
- `Referrer-Policy` — 控制 Referer 泄露（默认 `strict-origin-when-cross-origin`）
- `Permissions-Policy` — 浏览器功能权限控制（默认不设置）

## 快速开始

```go
import "github.com/Tsukikage7/servex/v2/middleware/secure"

// 使用默认配置（推荐生产环境）
mux.Handle("/", secure.HTTPMiddleware(nil)(myHandler))

// 或注入到 httpserver
srv := httpserver.New(mux,
    httpserver.WithMiddlewares(
        secure.HTTPMiddleware(nil),
    ),
)
```

## 自定义配置

```go
cfg := &secure.Config{
    XFrameOptions:         "SAMEORIGIN",                           // 允许同源 iframe
    ContentTypeNosniff:    true,
    XSSProtection:         "1; mode=block",
    HSTSMaxAge:            63072000,                               // 2 年
    HSTSIncludeSubdomains: true,
    HSTSPreload:           true,                                   // 加入 HSTS 预加载列表
    ContentSecurityPolicy: "default-src 'self'; img-src *",
    ReferrerPolicy:        "no-referrer",
    PermissionsPolicy:     "camera=(), microphone=()",
    IsDevelopment:         false,
}

mw := secure.HTTPMiddleware(cfg)
```

## 开发模式

设置 `IsDevelopment: true` 可跳过 HSTS，避免本地 HTTP 开发被强制重定向到 HTTPS：

```go
cfg := secure.DefaultConfig()
cfg.IsDevelopment = true
```

## Config 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `XFrameOptions` | `string` | `"DENY"` | 点击劫持防护 |
| `ContentTypeNosniff` | `bool` | `true` | MIME 嗅探防护 |
| `XSSProtection` | `string` | `"1; mode=block"` | XSS 过滤 |
| `HSTSMaxAge` | `int` | `31536000` | HSTS 有效期（秒），0 = 不设置 |
| `HSTSIncludeSubdomains` | `bool` | `true` | HSTS 含子域名 |
| `HSTSPreload` | `bool` | `false` | HSTS preload |
| `ContentSecurityPolicy` | `string` | `""` | CSP 策略，空 = 不设置 |
| `ReferrerPolicy` | `string` | `"strict-origin-when-cross-origin"` | Referrer 策略 |
| `PermissionsPolicy` | `string` | `""` | 权限策略，空 = 不设置 |
| `IsDevelopment` | `bool` | `false` | 开发模式（跳过 HSTS） |

## API

```go
// 创建 HTTP 中间件，cfg 为 nil 时使用 DefaultConfig
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler

// 返回默认安全配置
func DefaultConfig() *Config
```
