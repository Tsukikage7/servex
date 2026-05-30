package clientip

import (
	"net/http"
)

// HTTPMiddleware 创建 HTTP 客户端 IP 提取中间件.
//
// 提取优先级:
//  1. X-Forwarded-For header (取第一个非可信代理 IP)
//  2. X-Real-IP header
//  3. RemoteAddr
//
// 示例:
//
//	// 默认配置信任所有代理
//	handler = clientip.HTTPMiddleware()(handler)
//
//	// 只信任私有网络代理
//	handler = clientip.HTTPMiddleware(
//	    clientip.WithTrustPrivateProxies(),
//	)(handler)
//
//	// 指定可信代理
//	handler = clientip.HTTPMiddleware(
//	    clientip.WithTrustedProxies("10.0.0.0/8", "192.168.1.1"),
//	)(handler)
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractFromHTTP(r, o)
			ctx := WithIP(r.Context(), ip)

			// 如果配置了地理位置解析器
			if o.geoResolver != nil && ip.Address != "" {
				if geo, err := o.geoResolver.Lookup(ip.Address); err == nil && geo != nil {
					ctx = WithGeoInfo(ctx, geo)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractFromHTTP 从 HTTP 请求中提取客户端 IP.
func extractFromHTTP(r *http.Request, o *options) *IP {
	remoteIP := ParseIP(r.RemoteAddr)

	// 如果不信任所有代理，检查直接连接的客户端是否是可信代理
	if !o.trustAllProxies && !o.isTrustedProxy(remoteIP.Address) {
		// 直接客户端不是可信代理，不信任转发头
		return remoteIP
	}

	// 尝试从 X-Forwarded-For 提取
	if xff := r.Header.Get(o.forwardedHeader); xff != "" {
		var ipStr string
		if o.trustAllProxies {
			// 信任所有代理，取第一个 IP
			ipStr = ParseXForwardedFor(xff)
		} else {
			// 从右向左找第一个非可信代理
			ipStr = ParseXForwardedForWithTrust(xff, o.isTrustedProxy)
		}
		if ipStr != "" && IsValidIP(ipStr) {
			return &IP{Address: ipStr, Raw: xff}
		}
	}

	// 尝试从 X-Real-IP 提取
	if xri := r.Header.Get(o.realIPHeader); xri != "" {
		ipStr := ParseIP(xri).Address
		if ipStr != "" && IsValidIP(ipStr) {
			return &IP{Address: ipStr, Raw: xri}
		}
	}

	// 使用 RemoteAddr
	return remoteIP
}

// HTTPKeyFunc 返回用于限流等场景的 HTTP 键提取函数.
//
// 从 context 中获取已解析的客户端 IP.
// 需要配合 HTTPMiddleware 使用.
func HTTPKeyFunc() func(r *http.Request) string {
	return func(r *http.Request) string {
		return GetIP(r.Context())
	}
}
