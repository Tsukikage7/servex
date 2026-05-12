package tenant

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// HTTPMiddleware 返回 HTTP 租户解析中间件.
// 默认 TokenExtractor 为 BearerTokenExtractor().
// 示例:
//
//	handler = tenant.HTTPMiddleware(resolver)(handler)
func HTTPMiddleware(resolver Resolver, opts ...Option) func(http.Handler) http.Handler {
	if resolver == nil {
		panic("tenant: 解析器不能为空")
	}

	o := applyOptions(opts)

	if o.tokenExtractor == nil {
		o.tokenExtractor = BearerTokenExtractor()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 检查是否跳过
			if o.skipper != nil && o.skipper(ctx, r) {
				next.ServeHTTP(w, r)
				return
			}

			// 提取令牌
			token, err := o.tokenExtractor(ctx, r)
			if err != nil {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Debug("[Tenant] HTTP 令牌提取失败",
						logger.String("path", r.URL.Path),
						logger.Err(err),
					)
				}
				writeHTTPError(w, http.StatusUnauthorized, "tenant token required")
				return
			}

			// 解析租户
			t, err := resolver.Resolve(ctx, token)
			if err != nil {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Warn("[Tenant] HTTP 解析失败",
						logger.String("path", r.URL.Path),
						logger.Err(err),
					)
				}
				writeHTTPError(w, http.StatusUnauthorized, "invalid tenant")
				return
			}

			// 检查租户是否启用
			if !t.TenantEnabled() {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Warn("[Tenant] HTTP 租户已禁用",
						logger.String("tenant_id", t.TenantID()),
						logger.String("path", r.URL.Path),
					)
				}
				writeHTTPError(w, http.StatusForbidden, "tenant disabled")
				return
			}

			ctx = WithTenant(ctx, t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HTTPSkipPaths 返回跳过指定路径的 Skipper（精确匹配 + 通配前缀）.
// 示例:
//
//	tenant.HTTPSkipPaths("/health", "/api/public/*")
func HTTPSkipPaths(paths ...string) Skipper {
	exact := make(map[string]bool)
	var prefixes []string

	for _, p := range paths {
		if len(p) > 0 && p[len(p)-1] == '*' {
			prefixes = append(prefixes, p[:len(p)-1])
		} else {
			exact[p] = true
		}
	}

	return func(_ context.Context, request any) bool {
		r, ok := request.(*http.Request)
		if !ok {
			return false
		}
		if exact[r.URL.Path] {
			return true
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				return true
			}
		}
		return false
	}
}

// writeHTTPError 写入 HTTP 错误响应（使用 json.Marshal 防止 JSON 注入）.
func writeHTTPError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(body)
}
