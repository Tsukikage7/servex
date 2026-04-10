package circuitbreaker

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor 创建 gRPC 一元熔断器拦截器.
// 熔断器开路时返回 Unavailable 错误.
func UnaryServerInterceptor(cb CircuitBreaker) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var resp any
		err := cb.Execute(ctx, func() error {
			var e error
			resp, e = handler(ctx, req)
			return e
		})
		if errors.Is(err, ErrCircuitOpen) {
			return nil, status.Error(codes.Unavailable, "服务暂时不可用，请稍后重试")
		}
		return resp, err
	}
}

// StreamServerInterceptor 创建 gRPC 流熔断器拦截器.
func StreamServerInterceptor(cb CircuitBreaker) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := cb.Execute(ss.Context(), func() error {
			return handler(srv, ss)
		})
		if errors.Is(err, ErrCircuitOpen) {
			return status.Error(codes.Unavailable, "服务暂时不可用，请稍后重试")
		}
		return err
	}
}
