// Package secure 提供 HTTP 安全头中间件.
// 自动为每个响应设置常见的安全相关头部，防御点击劫持、MIME 嗅探、XSS 等攻击.
package secure

import (
	"fmt"
	"net/http"
)

// Config 安全头配置.
type Config struct {
	// XFrameOptions 防止点击劫持，默认 "DENY"
	XFrameOptions string
	// ContentTypeNosniff 防止 MIME 类型嗅探，默认 true
	ContentTypeNosniff bool
	// XSSProtection XSS 保护头，默认 "1; mode=block"
	XSSProtection string
	// HSTSMaxAge HSTS 最大有效期秒，0 表示不设置，默认 315360001年
	HSTSMaxAge int
	// HSTSIncludeSubdomains HSTS 包含子域名，默认 true
	HSTSIncludeSubdomains bool
	// HSTSPreload HSTS 预加载，默认 false
	HSTSPreload bool
	// ContentSecurityPolicy CSP 策略，默认空不设置
	ContentSecurityPolicy string
	// ReferrerPolicy 引用策略，默认 "strict-origin-when-cross-origin"
	ReferrerPolicy string
	// PermissionsPolicy 权限策略，默认空不设置
	PermissionsPolicy string
	// IsDevelopment 开发模式下跳过 HSTS，默认 false
	IsDevelopment bool
}

// DefaultConfig 返回默认安全头配置.
func DefaultConfig() *Config {
	return &Config{
		XFrameOptions:         "DENY",
		ContentTypeNosniff:    true,
		XSSProtection:         "1; mode=block",
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}
}

// HTTPMiddleware 创建安全头 HTTP 中间件.
// 在每个响应中设置配置的安全头部.
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 预构建 HSTS 值，避免每次请求重复拼接
	hstsValue := buildHSTSValue(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// X-Frame-Options
			if cfg.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.XFrameOptions)
			}

			// X-Content-Type-Options
			if cfg.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// X-XSS-Protection
			if cfg.XSSProtection != "" {
				w.Header().Set("X-XSS-Protection", cfg.XSSProtection)
			}

			// Strict-Transport-Security仅 HTTPS 请求且非开发模式下设置
			if hstsValue != "" && !cfg.IsDevelopment && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			// Content-Security-Policy
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}

			// Referrer-Policy
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// Permissions-Policy
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// buildHSTSValue 构建 HSTS 头部值.
func buildHSTSValue(cfg *Config) string {
	if cfg.HSTSMaxAge <= 0 {
		return ""
	}

	value := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
	if cfg.HSTSIncludeSubdomains {
		value += "; includeSubDomains"
	}
	if cfg.HSTSPreload {
		value += "; preload"
	}
	return value
}
