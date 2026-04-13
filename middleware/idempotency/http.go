package idempotency

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// HTTPMiddleware 返回 HTTP 幂等性中间件.
// 当请求携带 Idempotency-Key 请求头时，中间件会：
//  1. 检查该键是否已有结果
//  2. 如果有，直接返回之前的结果
//  3. 如果没有，执行请求并保存结果
//
// 默认只对 POST、PUT、PATCH 方法生效.
// 示例:
//
//	store := idempotency.NewRedisStore(redisClient)
//	handler = idempotency.HTTPMiddleware(store)(handler)
func HTTPMiddleware(store Store, opts ...Option) func(http.Handler) http.Handler {
	if store == nil {
		panic("idempotency: 存储实例不能为空")
	}

	o := applyOptions(store, opts)

	// 默认从 Idempotency-Key 请求头提取
	if o.keyExtractor == nil {
		o.keyExtractor = func(ctx any) string {
			if r, ok := ctx.(*http.Request); ok {
				return r.Header.Get(DefaultHTTPKeyHeader)
			}
			return ""
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 只对非幂等方法生效
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			// 提取幂等键
			key := o.keyExtractor(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// 检查是否已有结果
			result, err := store.Get(ctx, key)
			if err != nil {
				if o.skipOnError {
					if o.logger != nil {
						logger.FromContext(ctx).Warn(
							"[Idempotency] 存储获取失败，跳过检查",
							logger.String("key", key),
							logger.Err(err),
						)
					}
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if result != nil {
				// 返回之前的结果
				if o.logger != nil {
					logger.FromContext(ctx).Debug(
						"[Idempotency] 缓存命中",
						logger.String("key", key),
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
					)
				}
				writeHTTPResult(w, result)
				return
			}

			// 尝试获取处理锁
			locked, err := store.SetNX(ctx, key, o.lockTimeout)
			if err != nil {
				if o.skipOnError {
					if o.logger != nil {
						logger.FromContext(ctx).Warn(
							"[Idempotency] 获取锁失败，跳过检查",
							logger.String("key", key),
							logger.Err(err),
						)
					}
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !locked {
				// 请求正在处理中
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Request In Progress", http.StatusConflict)
				return
			}

			// 包装 ResponseWriter 以捕获响应
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			// 执行请求
			next.ServeHTTP(recorder, r)

			// 保存结果
			saveResult := &Result{
				StatusCode: recorder.statusCode,
				Headers:    make(map[string]string),
				Body:       recorder.body.Bytes(),
				CreatedAt:  time.Now(),
			}

			// 保存响应头
			for k, v := range recorder.Header() {
				if len(v) > 0 {
					saveResult.Headers[k] = v[0]
				}
			}

			if saveErr := store.Set(ctx, key, saveResult, o.ttl); saveErr != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Warn(
						"[Idempotency] 存储写入失败",
						logger.String("key", key),
						logger.Err(saveErr),
					)
				}
			}
		})
	}
}

// responseRecorder 记录 HTTP 响应.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// writeHTTPResult 写入缓存的 HTTP 结果.
func writeHTTPResult(w http.ResponseWriter, result *Result) {
	// 设置响应头
	for k, v := range result.Headers {
		w.Header().Set(k, v)
	}

	// 设置状态码
	w.WriteHeader(result.StatusCode)

	// 写入响应体
	if len(result.Body) > 0 {
		_, _ = io.Copy(w, bytes.NewReader(result.Body))
	}
}
