package logging

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// UnaryServerInterceptor 返回记录一元 RPC 请求日志的拦截器.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	o := applyOptions(opts)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if shouldSkip(info.FullMethod, o.SkipPaths) {
			return handler(ctx, req)
		}

		start := time.Now()
		resp, err := handler(ctx, req)

		logger.FromContextOr(ctx, o.Logger).Info("[gRPC] 请求完成",
			logger.String("method", info.FullMethod),
			logger.String("code", status.Code(err).String()),
			logger.String("duration", time.Since(start).String()),
		)
		return resp, err
	}
}

// StreamServerInterceptor 返回记录流式 RPC 请求日志的拦截器.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	o := applyOptions(opts)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if shouldSkip(info.FullMethod, o.SkipPaths) {
			return handler(srv, ss)
		}

		start := time.Now()
		err := handler(srv, ss)

		logger.FromContextOr(ss.Context(), o.Logger).Info("[gRPC] 流式请求完成",
			logger.String("method", info.FullMethod),
			logger.String("code", status.Code(err).String()),
			logger.String("duration", time.Since(start).String()),
		)
		return err
	}
}
