package logging

import (
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// HTTPMiddleware 返回 HTTP 请求日志中间件.
// 每次请求结束后记录方法、路径、状态码、耗时和响应字节数.
// 可通过 WithSkipPaths 跳过探活等不需要记录的路径.
// 示例:
//
//	handler = logging.HTTPMiddleware(
//	    logging.WithLogger(log),
//	    logging.WithSkipPaths("/health", "/metrics"),
//	)(handler)
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := applyOptions(opts)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkip(r.URL.Path, o.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(rec, r)

			logger.ForOr(r.Context(), o.Logger, "HTTP").Info("请求完成",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.Int("status", rec.status),
				logger.String("duration", time.Since(start).String()),
				logger.Int("bytes", rec.bytesWritten),
			)
		})
	}
}
