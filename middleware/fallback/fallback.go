// Package fallback 提供优雅降级中间件.
// 当下游服务失败（返回 5xx）或发生 panic 时，自动返回缓存或默认响应.
package fallback

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Options 配置选项.
type Options struct {
	// FallbackHandler 降级处理器.
	// 当主处理器返回 5xx 或 panic 时调用.
	FallbackHandler http.Handler

	// Logger 日志记录器.
	Logger logger.Logger
}

// Option 是配置函数.
type Option func(*Options)

// WithFallbackHandler 设置降级处理器.
func WithFallbackHandler(h http.Handler) Option {
	return func(o *Options) {
		o.FallbackHandler = h
	}
}

// WithFallbackFunc 设置降级处理函数.
func WithFallbackFunc(fn func(http.ResponseWriter, *http.Request)) Option {
	return func(o *Options) {
		o.FallbackHandler = http.HandlerFunc(fn)
	}
}

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// defaultOptions 返回默认配置.
func defaultOptions() *Options {
	return &Options{}
}

// applyOptions 应用配置选项.
func applyOptions(opts []Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// responseRecorder 用于捕获下游响应状态码.
// 在写入响应头之前缓冲状态码，以便判断是否需要降级.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader 捕获状态码.
func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.written = true
	// 如果是 5xx，不向客户端写入，等待降级处理
	if code >= 500 {
		return
	}
	r.ResponseWriter.WriteHeader(code)
}

// Write 写入响应体.
func (r *responseRecorder) Write(b []byte) (int, error) {
	// 如果状态码是 5xx，丢弃响应体
	if r.statusCode >= 500 {
		return len(b), nil
	}
	if !r.written {
		r.statusCode = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

// HTTPMiddleware 返回 HTTP 优雅降级中间件.
// 当下游 handler 返回 5xx 状态码或发生 panic 时，调用降级处理器.
// 如果未配置降级处理器，返回 503 Service Unavailable.
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := applyOptions(opts)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 捕获 panic
			panicked := true

			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			func() {
				defer func() {
					if p := recover(); p != nil {
						if o.Logger != nil {
							o.Logger.Error("[Fallback] 下游处理器发生 panic",
								logger.Any("panic", p),
								logger.String("method", r.Method),
								logger.String("path", r.URL.Path),
							)
						}
					}
				}()

				next.ServeHTTP(rec, r)
				panicked = false
			}()

			// 判断是否需要降级
			needFallback := panicked || rec.statusCode >= 500

			if needFallback {
				if o.Logger != nil && !panicked {
					o.Logger.Warn("[Fallback] 下游返回错误，触发降级",
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.Any("status", rec.statusCode),
					)
				}

				if o.FallbackHandler != nil {
					o.FallbackHandler.ServeHTTP(w, r)
					return
				}

				// 默认降级响应
				http.Error(w, "服务暂时不可用", http.StatusServiceUnavailable)
				return
			}
		})
	}
}
