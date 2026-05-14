# middleware/csrf

CSRF（跨站请求伪造）防护中间件，采用 **Double Submit Cookie** 模式：

- **安全方法**（GET/HEAD/OPTIONS/TRACE）：生成 token 并通过 cookie 下发，同时注入 context
- **非安全方法**（POST/PUT/DELETE/PATCH）：验证请求中的 token（header 或表单字段）与 cookie 一致
- 使用 `crypto/rand` 生成 token，`crypto/subtle.ConstantTimeCompare` 进行比对，防止时序攻击

## 快速开始

```go
import "github.com/Tsukikage7/servex/v2/middleware/csrf"

// 使用默认配置
mw := csrf.HTTPMiddleware(nil)

srv := httpserver.New(mux,
    httpserver.WithMiddlewares(mw),
)
```

## 在前端读取并回传 Token

```go
// 服务端：在 handler 中从 context 获取当前 CSRF token
func myHandler(w http.ResponseWriter, r *http.Request) {
    token := csrf.TokenFromContext(r.Context())
    // 将 token 写入页面模板或 JSON 响应，供前端使用
    json.NewEncoder(w).Encode(map[string]string{"csrf_token": token})
}
```

前端将 token 放入请求 header：
```
X-CSRF-Token: <token>
```

或表单字段：
```html
<input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
```

## 自定义配置

```go
cfg := &csrf.Config{
    TokenLength:  32,
    CookieName:   "_csrf",
    HeaderName:   "X-CSRF-Token",
    FormField:    "csrf_token",
    CookiePath:   "/",
    CookieMaxAge: 12 * time.Hour,
    Secure:       true,                    // HTTPS only
    HttpOnly:     true,                    // 禁止 JS 访问 cookie
    SameSite:     http.SameSiteStrictMode,

    // 跳过某些路径（如 webhook 回调）
    Skipper: func(r *http.Request) bool {
        return strings.HasPrefix(r.URL.Path, "/webhook")
    },

    // 自定义错误处理（默认返回 403）
    ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
        http.Error(w, "CSRF validation failed", http.StatusForbidden)
    },
}

mw := csrf.HTTPMiddleware(cfg)
```

## Config 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TokenLength` | `int` | `32` | token 长度（字节），编码后为 64 字符 hex |
| `CookieName` | `string` | `"_csrf"` | cookie 名称 |
| `HeaderName` | `string` | `"X-CSRF-Token"` | 请求 header 名 |
| `FormField` | `string` | `"csrf_token"` | 表单字段名（header 未提供时使用） |
| `CookiePath` | `string` | `"/"` | cookie 路径 |
| `CookieMaxAge` | `time.Duration` | `12h` | cookie 有效期 |
| `Secure` | `bool` | `true` | 仅 HTTPS 发送 cookie |
| `HttpOnly` | `bool` | `true` | 禁止 JS 访问 cookie |
| `SameSite` | `http.SameSite` | `Strict` | SameSite 属性 |
| `Skipper` | `func(*http.Request) bool` | `nil` | 返回 true 时跳过验证 |
| `ErrorHandler` | `func(w, r, err)` | `nil` | 自定义错误响应（默认 403） |

## 预定义错误

```go
csrf.ErrMissingToken  // cookie 或请求中缺少 token
csrf.ErrInvalidToken  // token 不匹配
```

## API

```go
// 创建 HTTP 中间件，cfg 为 nil 时使用 DefaultConfig
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler

// 从 context 中获取当前请求的 CSRF token（供模板或 JSON 响应使用）
func TokenFromContext(ctx context.Context) string

// 返回默认 CSRF 配置
func DefaultConfig() *Config
```
