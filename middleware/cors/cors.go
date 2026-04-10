// Package cors 提供 HTTP CORS（跨源资源共享）中间件.
package cors

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Options CORS 配置.
type Options struct {
	// AllowOrigins 允许的来源列表，["*"] 表示允许所有来源.
	AllowOrigins []string
	// AllowMethods 允许的 HTTP 方法.
	AllowMethods []string
	// AllowHeaders 允许的请求头.
	AllowHeaders []string
	// ExposeHeaders 允许客户端读取的响应头.
	ExposeHeaders []string
	// AllowCredentials 是否允许携带凭据（Cookie、Authorization 等）.
	// AllowOrigins 为 "*" 时此字段不生效.
	AllowCredentials bool
	// MaxAge 预检结果缓存时间（秒），默认 86400（24 小时）.
	MaxAge int
}

// Option 配置函数.
type Option func(*Options)

// WithAllowOrigins 设置允许的来源.
func WithAllowOrigins(origins ...string) Option {
	return func(o *Options) { o.AllowOrigins = origins }
}

// WithAllowMethods 设置允许的 HTTP 方法.
func WithAllowMethods(methods ...string) Option {
	return func(o *Options) { o.AllowMethods = methods }
}

// WithAllowHeaders 设置允许的请求头.
func WithAllowHeaders(headers ...string) Option {
	return func(o *Options) { o.AllowHeaders = headers }
}

// WithExposeHeaders 设置可暴露给客户端的响应头.
func WithExposeHeaders(headers ...string) Option {
	return func(o *Options) { o.ExposeHeaders = headers }
}

// WithAllowCredentials 设置是否允许携带凭据.
func WithAllowCredentials(allow bool) Option {
	return func(o *Options) { o.AllowCredentials = allow }
}

// WithMaxAge 设置预检结果缓存时间（秒）.
func WithMaxAge(seconds int) Option {
	return func(o *Options) { o.MaxAge = seconds }
}

// defaultOptions 默认配置.
func defaultOptions() Options {
	return Options{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
		},
		MaxAge: 86400,
	}
}

// HTTPMiddleware 创建 HTTP CORS 中间件.
// 处理逻辑：
//  1. 无 Origin 头 → 直接透传（非 CORS 请求）
//  2. 验证来源是否在白名单
//  3. 写 Access-Control-* 响应头
//  4. OPTIONS 预检请求 → 返回 204 短路
//  5. 其他方法 → 透传给下一个 handler
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	// 安全警告：AllowOrigins 为 ["*"] 允许任意来源跨域访问，
	// 在生产环境应明确指定允许的来源列表.
	if isAllOrigins(o.AllowOrigins) {
		log.Println("cors: WARNING AllowOrigins is set to wildcard \"*\", all origins are allowed. Consider setting explicit origins for production use.")
	}

	// 安全警告：AllowCredentials 与 AllowOrigins "*" 冲突
	// 浏览器不允许 Access-Control-Allow-Origin: * 与 Access-Control-Allow-Credentials: true 同时使用.
	if o.AllowCredentials && isAllOrigins(o.AllowOrigins) {
		log.Println("cors: WARNING AllowCredentials=true 与 AllowOrigins=[\"*\"] 冲突，浏览器将拒绝携带凭据的跨域请求。请设置明确的 AllowOrigins 列表。")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 非 CORS 请求直接透传
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 验证来源
			if !isOriginAllowed(origin, o.AllowOrigins) {
				next.ServeHTTP(w, r)
				return
			}

			// 写 CORS 响应头
			if isAllOrigins(o.AllowOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// 告知缓存此响应因 Origin 不同而变化
				w.Header().Add("Vary", "Origin")
			}

			if o.AllowCredentials && !isAllOrigins(o.AllowOrigins) {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(o.ExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(o.ExposeHeaders, ", "))
			}

			// 预检请求处理
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(o.AllowMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(o.AllowHeaders, ", "))
				if o.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(o.MaxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed 检查 origin 是否在白名单中.
// 支持精确匹配和子域名通配符（如 "*.example.com"）.
func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
		// 支持子域名通配符，如 "*.example.com" 匹配 "foo.example.com"
		if strings.HasPrefix(a, "*.") {
			// 提取通配符的域名后缀，如 "*.example.com" -> ".example.com"
			suffix := a[1:] // ".example.com"
			if strings.HasSuffix(strings.ToLower(origin), strings.ToLower(suffix)) {
				// 确保 origin 中包含协议头，提取 host 部分
				// origin 格式通常为 "https://sub.example.com"
				if idx := strings.Index(origin, "://"); idx != -1 {
					host := origin[idx+3:]
					if strings.HasSuffix(strings.ToLower(host), strings.ToLower(suffix)) {
						return true
					}
				}
			}
		}
	}
	return false
}

// isAllOrigins 检查是否允许所有来源.
func isAllOrigins(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
