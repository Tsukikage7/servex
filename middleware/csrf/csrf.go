// Package csrf 提供 CSRF跨站请求伪造防护中间件.
// 通过 Double Submit Cookie 模式防御 CSRF 攻击：
//   - 安全方法GET/HEAD/OPTIONS/TRACE生成 token 并通过 cookie 下发
//   - 非安全方法POST/PUT/DELETE/PATCH验证请求中的 token 与 cookie 一致
//   - 使用 crypto/rand 生成 token，crypto/subtle.ConstantTimeCompare 进行比对
package csrf

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

var (
	// ErrMissingToken 缺少 CSRF token.
	ErrMissingToken = errors.New("csrf: missing CSRF token")
	// ErrInvalidToken CSRF token 无效.
	ErrInvalidToken = errors.New("csrf: invalid CSRF token")
)

// Config CSRF 配置.
type Config struct {
	// TokenLength token 长度字节，默认 32
	TokenLength int
	// CookieName cookie 名，默认 "_csrf"
	CookieName string
	// HeaderName header 名，默认 "X-CSRF-Token"
	HeaderName string
	// FormField 表单字段名，默认 "csrf_token"
	FormField string
	// CookiePath cookie 路径，默认 "/"
	CookiePath string
	// CookieMaxAge cookie 最大有效期，默认 12h
	CookieMaxAge time.Duration
	// Secure HTTPS only，默认 true
	Secure bool
	// HttpOnly 防止 JS 访问，默认 true
	HttpOnly bool
	// SameSite SameSite 属性，默认 Strict
	SameSite http.SameSite
	// Skipper 跳过函数
	Skipper func(r *http.Request) bool
	// ErrorHandler 自定义错误处理
	ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)
}

// DefaultConfig 返回默认 CSRF 配置.
func DefaultConfig() *Config {
	return &Config{
		TokenLength:  32,
		CookieName:   "_csrf",
		HeaderName:   "X-CSRF-Token",
		FormField:    "csrf_token",
		CookiePath:   "/",
		CookieMaxAge: 12 * time.Hour,
		Secure:       true,
		HttpOnly:     true,
		SameSite:     http.SameSiteStrictMode,
	}
}

type ctxKey struct{}

// TokenFromContext 从 context 中获取当前请求的 CSRF token.
func TokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

// HTTPMiddleware 创建 CSRF 防护 HTTP 中间件.
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 跳过判断
			if cfg.Skipper != nil && cfg.Skipper(r) {
				next.ServeHTTP(w, r)
				return
			}

			if isSafeMethod(r.Method) {
				// 安全方法：检查是否已有有效 token，避免每次请求都重新生成支持多标签页
				token := ""
				if cookie, err := r.Cookie(cfg.CookieName); err == nil && cookie.Value != "" {
					// cookie 中已有 token，检查长度和 hex 格式是否正确
					if len(cookie.Value) == cfg.TokenLength*2 && isValidHex(cookie.Value) {
						token = cookie.Value
					}
				}

				// 无有效 token 时才生成新 token
				if token == "" {
					var err error
					token, err = generateToken(cfg.TokenLength)
					if err != nil {
						handleError(w, r, cfg, err)
						return
					}

					http.SetCookie(w, &http.Cookie{
						Name:     cfg.CookieName,
						Value:    token,
						Path:     cfg.CookiePath,
						MaxAge:   int(cfg.CookieMaxAge.Seconds()),
						Secure:   cfg.Secure,
						HttpOnly: cfg.HttpOnly,
						SameSite: cfg.SameSite,
					})
				}

				ctx := context.WithValue(r.Context(), ctxKey{}, token)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 非安全方法：验证 token
			cookieToken, err := r.Cookie(cfg.CookieName)
			if err != nil || cookieToken.Value == "" {
				handleError(w, r, cfg, ErrMissingToken)
				return
			}

			// 从 header 或表单获取请求 token
			requestToken := r.Header.Get(cfg.HeaderName)
			if requestToken == "" {
				requestToken = r.FormValue(cfg.FormField)
			}
			if requestToken == "" {
				handleError(w, r, cfg, ErrMissingToken)
				return
			}

			// 恒定时间比较，防止时序攻击
			if subtle.ConstantTimeCompare([]byte(cookieToken.Value), []byte(requestToken)) != 1 {
				handleError(w, r, cfg, ErrInvalidToken)
				return
			}

			// 将已验证的 token 注入 context
			ctx := context.WithValue(r.Context(), ctxKey{}, cookieToken.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isSafeMethod 判断是否为安全方法.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// generateToken 生成随机 token.
func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// handleError 处理错误.
func handleError(w http.ResponseWriter, r *http.Request, cfg *Config, err error) {
	if cfg.ErrorHandler != nil {
		cfg.ErrorHandler(w, r, err)
		return
	}
	http.Error(w, "CSRF 验证失败", http.StatusForbidden)
}

// isValidHex 检查字符串是否为合法的 hex 编码.
func isValidHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}
