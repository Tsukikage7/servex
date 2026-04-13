package recovery

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// HTTPMiddleware 返回 HTTP panic 恢复中间件.
// 当 handler 发生 panic 时，中间件会：
//  1. 捕获 panic 并记录堆栈信息
//  2. 调用自定义 Handler（如果设置）
//  3. 返回 500 Internal Server Error
//
// 示例:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", handler)
//	wrapped := recovery.HTTPMiddleware(recovery.WithLogger(log))(mux)
//	http.ListenAndServe(":8080", wrapped)
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := applyOptions(opts)
	if o.Logger == nil {
		panic("recovery: 日志记录器不能为空")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					stack := captureStack(o.StackSize, o.StackAll)

					// 记录 panic 日志
					logger.FromContext(r.Context()).Error(
						"http panic recovered",
						logger.Any("panic", p),
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.String("stack", string(stack)),
					)

					// 调用自定义处理函数
					if o.Handler != nil {
						_ = o.Handler(r, p, stack)
					}

					// 返回 500，写入响应体便于客户端识别错误
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// HTTPRecoverFunc 是简化版 HTTP 恢复函数，用于单个 handler.
// 示例:
//
//	http.HandleFunc("/", recovery.HTTPRecoverFunc(log, myHandler))
func HTTPRecoverFunc(l logger.Logger, handler http.HandlerFunc) http.HandlerFunc {
	return HTTPMiddleware(WithLogger(l))(http.HandlerFunc(handler)).ServeHTTP
}
