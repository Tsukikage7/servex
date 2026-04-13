package circuitbreaker

import (
	"context"

	"github.com/Tsukikage7/servex/v2/endpoint"
)

// EndpointMiddleware 创建 Endpoint 熔断器中间件.
// 熔断器开路时返回 ErrCircuitOpen 错误.
func EndpointMiddleware(cb CircuitBreaker) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			var resp any
			err := cb.Execute(ctx, func() error {
				var e error
				resp, e = next(ctx, request)
				return e
			})
			return resp, err
		}
	}
}
