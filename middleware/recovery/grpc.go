package recovery

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// UnaryServerInterceptor 返回 gRPC 一元服务器 panic 恢复拦截器.
// 当 handler 发生 panic 时，拦截器会：
//  1. 捕获 panic 并记录堆栈信息
//  2. 调用自定义 Handler（如果设置）
//  3. 返回 codes.Internal 错误
//
// 示例:
//
//	srv := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        recovery.UnaryServerInterceptor(recovery.WithLogger(log)),
//	    ),
//	)
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	o := applyOptions(opts)
	if o.Logger == nil {
		panic("recovery: 日志记录器不能为空")
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				stack := captureStack(o.StackSize, o.StackAll)

				// 记录 panic 日志
				logger.FromContext(ctx).Error(
					"grpc unary panic recovered",
					logger.Any("panic", p),
					logger.String("method", info.FullMethod),
					logger.String("stack", string(stack)),
				)

				// 调用自定义处理函数
				if o.Handler != nil {
					err = o.Handler(ctx, p, stack)
					return
				}

				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器 panic 恢复拦截器.
// 当 handler 发生 panic 时，拦截器会：
//  1. 捕获 panic 并记录堆栈信息
//  2. 调用自定义 Handler（如果设置）
//  3. 返回 codes.Internal 错误
//
// 示例:
//
//	srv := grpc.NewServer(
//	    grpc.ChainStreamInterceptor(
//	        recovery.StreamServerInterceptor(recovery.WithLogger(log)),
//	    ),
//	)
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	o := applyOptions(opts)
	if o.Logger == nil {
		panic("recovery: 日志记录器不能为空")
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if p := recover(); p != nil {
				stack := captureStack(o.StackSize, o.StackAll)

				// 记录 panic 日志
				logger.FromContext(ss.Context()).Error(
					"grpc stream panic recovered",
					logger.Any("panic", p),
					logger.String("method", info.FullMethod),
					logger.String("stream_type", streamType(info)),
					logger.String("stack", string(stack)),
				)

				// 调用自定义处理函数
				if o.Handler != nil {
					err = o.Handler(ss.Context(), p, stack)
					return
				}

				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(srv, ss)
	}
}

// streamType 返回流类型描述.
func streamType(info *grpc.StreamServerInfo) string {
	switch {
	case info.IsClientStream && info.IsServerStream:
		return "bidi"
	case info.IsClientStream:
		return "client"
	case info.IsServerStream:
		return "server"
	default:
		return "unary"
	}
}
