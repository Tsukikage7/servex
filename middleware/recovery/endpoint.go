package recovery

import (
	"context"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// EndpointMiddleware 返回 Endpoint panic 恢复中间件.
// 当 endpoint 发生 panic 时，中间件会：
//  1. 捕获 panic 并记录堆栈信息
//  2. 调用自定义 Handler（如果设置）
//  3. 返回 PanicError
//
// 示例:
//
//	endpoint := myEndpoint
//	endpoint = recovery.EndpointMiddleware(recovery.WithLogger(log))(endpoint)
func EndpointMiddleware(opts ...Option) endpoint.Middleware {
	o := applyOptions(opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (response any, err error) {
			defer func() {
				if p := recover(); p != nil {
					stack := captureStack(o.StackSize, o.StackAll)

					// 记录 panic 日志
					logger.FromContextOr(ctx, o.Logger).Error(
						"[Recovery] 端点异常已恢复",
						logger.Any("panic", p),
						logger.String("stack", string(stack)),
					)

					// 调用自定义处理函数
					if o.Handler != nil {
						err = o.Handler(ctx, p, stack)
						return
					}

					err = &PanicError{Value: p, Stack: stack}
				}
			}()

			return next(ctx, request)
		}
	}
}
