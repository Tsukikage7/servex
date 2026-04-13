package activity

import (
	"math/rand"
	"net/http"

	"github.com/Tsukikage7/servex/v2/httpx/clientip"
)

// HTTPMiddleware 返回 HTTP 中间件，自动追踪用户活跃.
func HTTPMiddleware(tracker *Tracker, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	o := &middlewareOptions{
		skipPaths: map[string]bool{
			"/health":      true,
			"/healthz":     true,
			"/ready":       true,
			"/readyz":      true,
			"/metrics":     true,
			"/favicon.ico": true,
		},
		eventType: EventTypeRequest,
	}
	for _, opt := range opts {
		opt(o)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 跳过指定路径
			if o.skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// 采样
			if tracker.opts.sampleRate < 1.0 && rand.Float64() > tracker.opts.sampleRate {
				next.ServeHTTP(w, r)
				return
			}

			// 提取用户 ID
			var userID string
			if tracker.opts.extractor != nil {
				userID = tracker.opts.extractor(r.Context())
			}

			// 构建事件
			if userID != "" {
				event := &Event{
					UserID:    userID,
					EventType: o.eventType,
					Path:      r.URL.Path,
					IP:        clientip.GetIP(r.Context()),
					Platform:  detectPlatform(r),
				}

				// 异步追踪，不阻塞请求
				_ = tracker.Track(r.Context(), event)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareOption 中间件配置选项.
type MiddlewareOption func(*middlewareOptions)

type middlewareOptions struct {
	skipPaths map[string]bool
	eventType EventType
}

// WithSkipPaths 设置跳过的路径.
func WithSkipPaths(paths ...string) MiddlewareOption {
	return func(o *middlewareOptions) {
		for _, p := range paths {
			o.skipPaths[p] = true
		}
	}
}

// WithEventType 设置事件类型.
func WithEventType(eventType EventType) MiddlewareOption {
	return func(o *middlewareOptions) {
		o.eventType = eventType
	}
}

// detectPlatform 从请求检测平台.
func detectPlatform(r *http.Request) string {
	// 优先使用 Client Hints
	if platform := r.Header.Get("Sec-CH-UA-Platform"); platform != "" {
		return trimQuotes(platform)
	}

	// 从 User-Agent 简单判断
	ua := r.Header.Get("User-Agent")
	switch {
	case contains(ua, "iPhone") || contains(ua, "iPad"):
		return "iOS"
	case contains(ua, "Android"):
		return "Android"
	case contains(ua, "Windows"):
		return "Windows"
	case contains(ua, "Mac OS"):
		return "macOS"
	case contains(ua, "Linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
