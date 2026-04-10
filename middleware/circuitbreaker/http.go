package circuitbreaker

import (
	"fmt"
	"net/http"
)

// responseRecorder 记录下游 handler 写入的状态码.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware 创建 HTTP 熔断器中间件.
// 熔断器开路时返回 503 Service Unavailable.
// 下游返回 >= 500 状态码视为失败，会触发熔断器计数.
func HTTPMiddleware(cb CircuitBreaker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := cb.Execute(r.Context(), func() error {
				rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
				next.ServeHTTP(rec, r)
				if rec.statusCode >= http.StatusInternalServerError {
					return fmt.Errorf("circuitbreaker: 下游返回状态码 %d", rec.statusCode)
				}
				return nil
			})
			if err != nil {
				// 熔断器开路，且下游尚未写入响应头
				http.Error(w, "服务暂时不可用，请稍后重试", http.StatusServiceUnavailable)
			}
		})
	}
}
