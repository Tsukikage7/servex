package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

// HTTPMiddleware 创建 HTTP 限流中间件.
// 当请求被限流时返回 429 Too Many Requests.
func HTTPMiddleware(limiter Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow(r.Context()) {
				http.Error(w, "请求过于频繁，请稍后重试", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HTTPMiddlewareWithWait 创建阻塞式 HTTP 限流中间件.
// 当请求被限流时阻塞等待，直到可以通过或请求超时.
func HTTPMiddlewareWithWait(limiter Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := limiter.Wait(r.Context()); err != nil {
				http.Error(w, "请求超时", http.StatusGatewayTimeout)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HTTPKeyFunc 用于从 HTTP 请求中提取限流键.
type HTTPKeyFunc func(r *http.Request) string

// KeyedHTTPMiddleware 创建基于键的 HTTP 限流中间件.
// 可以基于 IP 地址、用户 ID 等进行限流.
func KeyedHTTPMiddleware(keyFunc HTTPKeyFunc, getLimiter KeyedLimiterFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			limiter := getLimiter(key)
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.Allow(r.Context()) {
				http.Error(w, "请求过于频繁，请稍后重试", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPKeyFunc 返回基于客户端 IP 的键提取函数.
//
// 警告: 此函数直接信任 X-Forwarded-For 和 X-Real-IP 请求头，
// 在不受信任的代理后面使用时，客户端可以伪造这些头部来绕过限流.
// 如需在反向代理后安全使用，请改用 TrustedProxyIPKeyFunc.
func IPKeyFunc() HTTPKeyFunc {
	return func(r *http.Request) string {
		// 优先使用 X-Forwarded-For
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return xff
		}
		// 使用 X-Real-IP
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
		// 使用 RemoteAddr
		return r.RemoteAddr
	}
}

// TrustedProxyIPKeyFunc 返回仅信任指定代理 IP 转发头的客户端 IP 提取函数.
//
// 当请求来自受信任的代理时，才使用 X-Forwarded-For / X-Real-IP 头部提取客户端 IP.
// 否则直接使用 RemoteAddr，防止客户端伪造头部绕过限流.
//
// trustedProxies 为可信代理的 IP 地址或 CIDR 列表如 "10.0.0.0/8", "192.168.1.1".
func TrustedProxyIPKeyFunc(trustedProxies ...string) HTTPKeyFunc {
	// 预解析 CIDR 和 IP
	var cidrs []*net.IPNet
	var ips []net.IP
	for _, proxy := range trustedProxies {
		if _, cidr, err := net.ParseCIDR(proxy); err == nil {
			cidrs = append(cidrs, cidr)
		} else if ip := net.ParseIP(proxy); ip != nil {
			ips = append(ips, ip)
		}
	}

	isTrusted := func(addr string) bool {
		// 分离 host:port
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return false
		}
		for _, cidr := range cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
		for _, trusted := range ips {
			if trusted.Equal(ip) {
				return true
			}
		}
		return false
	}

	return func(r *http.Request) string {
		if isTrusted(r.RemoteAddr) {
			// 来自受信任代理，取 X-Forwarded-For 最左侧原始客户端 IP
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.SplitN(xff, ",", 2)
				return strings.TrimSpace(parts[0])
			}
			if xri := r.Header.Get("X-Real-IP"); xri != "" {
				return strings.TrimSpace(xri)
			}
		}
		// 非受信任代理或无转发头，直接使用 RemoteAddr
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}

// PathKeyFunc 返回基于请求路径的键提取函数.
func PathKeyFunc() HTTPKeyFunc {
	return func(r *http.Request) string {
		return r.URL.Path
	}
}

// CompositeKeyFunc 组合多个键提取函数.
func CompositeKeyFunc(funcs ...HTTPKeyFunc) HTTPKeyFunc {
	return func(r *http.Request) string {
		var key string
		for _, f := range funcs {
			if key != "" {
				key += ":"
			}
			key += f(r)
		}
		return key
	}
}
