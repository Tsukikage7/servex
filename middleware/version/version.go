// Package version 提供 API 版本控制中间件.
// 支持从 URL 路径前缀/v1/、/v2/或请求头Accept-Version、API-Version提取版本信息.
package version

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

// contextKey context 键类型.
type contextKey struct{}

// versionKey 用于在 context 中存储 API 版本.
var versionKey = contextKey{}

// Options 配置选项.
type Options struct {
	// PathPrefix 是否从 URL 路径前缀提取版本默认 true.
	PathPrefix bool

	// HeaderName 从指定请求头提取版本.
	// 默认依次检查 Accept-Version 和 API-Version.
	HeaderName string

	// DefaultVersion 默认版本，当无法提取版本时使用.
	DefaultVersion string
}

// Option 是配置函数.
type Option func(*Options)

// WithPathPrefix 设置是否从 URL 路径前缀提取版本.
func WithPathPrefix(enabled bool) Option {
	return func(o *Options) {
		o.PathPrefix = enabled
	}
}

// WithHeader 设置自定义版本请求头名称.
func WithHeader(name string) Option {
	return func(o *Options) {
		o.HeaderName = name
	}
}

// WithDefault 设置默认版本.
func WithDefault(v string) Option {
	return func(o *Options) {
		o.DefaultVersion = v
	}
}

// defaultOptions 返回默认配置.
func defaultOptions() *Options {
	return &Options{
		PathPrefix:     true,
		DefaultVersion: "",
	}
}

// applyOptions 应用配置选项.
func applyOptions(opts []Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// pathVersionRegex 匹配路径中的版本前缀，如 /v1/、/v2/.
var pathVersionRegex = regexp.MustCompile(`^/v(\d+)(?:/|$)`)

// HTTPMiddleware 返回 HTTP API 版本提取中间件.
// 从请求中提取 API 版本并存储到 context 中.
// 提取优先级：URL 路径前缀 > 自定义请求头 > Accept-Version > API-Version > 默认版本.
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := applyOptions(opts)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ver := ""

			// 1. 从 URL 路径前缀提取
			if o.PathPrefix {
				if matches := pathVersionRegex.FindStringSubmatch(r.URL.Path); len(matches) > 1 {
					ver = "v" + matches[1]
					// 去掉路径中的版本前缀，便于后续路由匹配
					trimmed := strings.TrimPrefix(r.URL.Path, "/"+ver)
					if trimmed == "" {
						trimmed = "/"
					}
					r.URL.Path = trimmed
				}
			}

			// 2. 从自定义请求头提取
			if ver == "" && o.HeaderName != "" {
				ver = r.Header.Get(o.HeaderName)
			}

			// 3. 从标准请求头提取
			if ver == "" {
				if v := r.Header.Get("Accept-Version"); v != "" {
					ver = v
				} else if v := r.Header.Get("API-Version"); v != "" {
					ver = v
				}
			}

			// 4. 使用默认版本
			if ver == "" {
				ver = o.DefaultVersion
			}

			// 存入 context
			ctx := context.WithValue(r.Context(), versionKey, ver)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext 从 context 中获取 API 版本.
// 如果 context 中没有版本信息，返回空字符串.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(versionKey).(string); ok {
		return v
	}
	return ""
}
