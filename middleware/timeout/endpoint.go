package timeout

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/endpoint"
	"github.com/Tsukikage7/servex/observability/logger"
)

// EndpointMiddleware 返回 Endpoint 超时控制中间件.
// 当请求超时时，中间件会：
//  1. 取消 context
//  2. 记录超时日志（如果设置了 logger）
//  3. 调用超时回调（如果设置了 onTimeout）
//  4. 返回 ErrTimeout 或 context.DeadlineExceeded
//
// 示例:
//
//	endpoint := myEndpoint
//	endpoint = timeout.EndpointMiddleware(5*time.Second)(endpoint)
//
// 带日志:
//
//	endpoint = timeout.EndpointMiddleware(5*time.Second,
//	    timeout.WithLogger(log),
//	)(endpoint)
func EndpointMiddleware(timeout time.Duration, opts ...Option) endpoint.Middleware {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(timeout, opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 创建带超时的 context
			ctx, cancel := Cascade(ctx, o.timeout)
			defer cancel()

			// 使用 channel 等待结果
			type result struct {
				response any
				err      error
			}
			done := make(chan result, 1)

			go func() {
				resp, err := next(ctx, request)
				done <- result{response: resp, err: err}
			}()

			select {
			case <-ctx.Done():
				// 超时或取消
				if o.logger != nil {
					logger.FromContext(ctx).Warn(
						"[Timeout] 端点执行超时",
						logger.Duration("timeout", o.timeout),
					)
				}
				if o.onTimeout != nil {
					o.onTimeout(ctx, o.timeout)
				}
				return nil, ctx.Err()

			case r := <-done:
				return r.response, r.err
			}
		}
	}
}

// EndpointMiddlewareWithFallback 返回带降级的超时中间件.
// 当请求超时时，调用 fallback 函数返回降级响应，而不是返回错误.
// 示例:
//
//	endpoint = timeout.EndpointMiddlewareWithFallback(
//	    5*time.Second,
//	    func(ctx context.Context, request any) (any, error) {
//	        return &DefaultResponse{}, nil
//	    },
//	)(endpoint)
func EndpointMiddlewareWithFallback(
	timeout time.Duration,
	fallback endpoint.Endpoint,
	opts ...Option,
) endpoint.Middleware {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}
	if fallback == nil {
		panic("timeout: 降级函数不能为空")
	}

	o := applyOptions(timeout, opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			ctx, cancel := Cascade(ctx, o.timeout)
			defer cancel()

			type result struct {
				response any
				err      error
			}
			done := make(chan result, 1)

			go func() {
				resp, err := next(ctx, request)
				done <- result{response: resp, err: err}
			}()

			select {
			case <-ctx.Done():
				if o.logger != nil {
					logger.FromContext(ctx).Warn(
						"[Timeout] 端点执行超时，使用降级响应",
						logger.Duration("timeout", o.timeout),
					)
				}
				if o.onTimeout != nil {
					o.onTimeout(ctx, o.timeout)
				}
				// 使用新的 context 调用 fallback，避免使用已取消的 context
				return fallback(context.Background(), request)

			case r := <-done:
				return r.response, r.err
			}
		}
	}
}
