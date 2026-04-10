package timeout

import (
	"context"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/observability/logger"
)

// HTTPMiddleware 返回 HTTP 超时控制中间件.
// 当请求超时时，中间件会：
//  1. 取消请求 context
//  2. 记录超时日志（如果设置了 logger）
//  3. 调用超时回调（如果设置了 onTimeout）
//  4. 返回 503 Service Unavailable
//
// 注意: 此中间件不会中断正在执行的 handler，只是不再等待其响应.
// 如果需要强制中断，handler 应该检查 ctx.Done().
// 示例:
//
//	mux := http.NewServeMux()
//	handler := timeout.HTTPMiddleware(10*time.Second)(mux)
//	http.ListenAndServe(":8080", handler)
func HTTPMiddleware(timeout time.Duration, opts ...Option) func(http.Handler) http.Handler {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(timeout, opts)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := Cascade(r.Context(), o.timeout)
			defer cancel()

			// 使用带超时的 context
			r = r.WithContext(ctx)

			// 包装 ResponseWriter 以检测是否已写入响应
			tw := &timeoutWriter{
				ResponseWriter: w,
				done:           make(chan struct{}),
			}

			// 在 goroutine 中执行 handler
			go func() {
				defer close(tw.done)
				next.ServeHTTP(tw, r)
			}()

			select {
			case <-ctx.Done():
				// 超时
				tw.mu.Lock()
				defer tw.mu.Unlock()

				if !tw.written {
					// 还没写入响应，返回超时错误
					if o.logger != nil {
						logger.FromContext(ctx).Warn(
							"[Timeout] HTTP请求超时",
							logger.String("method", r.Method),
							logger.String("path", r.URL.Path),
							logger.Duration("timeout", o.timeout),
						)
					}
					if o.onTimeout != nil {
						o.onTimeout(r, o.timeout)
					}

					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte("Service Unavailable: request timeout"))
				}

			case <-tw.done:
				// handler 正常完成
			}
		})
	}
}

// timeoutWriter 包装 http.ResponseWriter 以跟踪写入状态.
type timeoutWriter struct {
	http.ResponseWriter
	mu      mutex
	written bool
	done    chan struct{}
}

// mutex 简单互斥锁（避免导入 sync 包只为一个类型）.
type mutex struct {
	ch chan struct{}
}

func (m *mutex) Lock() {
	if m.ch == nil {
		m.ch = make(chan struct{}, 1)
	}
	m.ch <- struct{}{}
}

func (m *mutex) Unlock() {
	<-m.ch
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.written {
		return
	}
	tw.written = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if !tw.written {
		tw.written = true
	}
	return tw.ResponseWriter.Write(b)
}

// HTTPTimeoutHandler 返回带超时的 http.Handler.
// 这是标准库 http.TimeoutHandler 的增强版，支持日志记录和回调.
// 示例:
//
//	handler := timeout.HTTPTimeoutHandler(myHandler, 10*time.Second, "请求超时")
func HTTPTimeoutHandler(h http.Handler, dt time.Duration, msg string, opts ...Option) http.Handler {
	if dt <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(dt, opts)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dt)
		defer cancel()

		r = r.WithContext(ctx)

		done := make(chan struct{})
		tw := &timeoutWriter{
			ResponseWriter: w,
			done:           done,
		}

		go func() {
			defer close(done)
			h.ServeHTTP(tw, r)
		}()

		select {
		case <-done:
			// 正常完成
		case <-ctx.Done():
			tw.mu.Lock()
			defer tw.mu.Unlock()

			if !tw.written {
				if o.logger != nil {
					logger.FromContext(ctx).Warn(
						"[Timeout] HTTP处理器超时",
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.Duration("timeout", dt),
					)
				}
				if o.onTimeout != nil {
					o.onTimeout(r, dt)
				}

				w.WriteHeader(http.StatusServiceUnavailable)
				if msg != "" {
					_, _ = w.Write([]byte(msg))
				}
			}
		}
	})
}
