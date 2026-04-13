package timeout

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

// UnaryServerInterceptor 返回 gRPC 一元服务器超时拦截器.
// 当请求超时时，拦截器会：
//  1. 取消请求 context
//  2. 记录超时日志（如果设置了 logger）
//  3. 调用超时回调（如果设置了 onTimeout）
//  4. 返回 codes.DeadlineExceeded 错误
//
// 示例:
//
//	srv := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        timeout.UnaryServerInterceptor(5*time.Second),
//	    ),
//	)
func UnaryServerInterceptor(timeout time.Duration, opts ...Option) grpc.UnaryServerInterceptor {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(timeout, opts)

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx, cancel := Cascade(ctx, o.timeout)
		defer cancel()

		type result struct {
			resp any
			err  error
		}
		done := make(chan result, 1)

		go func() {
			resp, err := handler(ctx, req)
			done <- result{resp: resp, err: err}
		}()

		select {
		case <-ctx.Done():
			if o.logger != nil {
				logger.FromContext(ctx).Warn(
					"[Timeout] gRPC一元请求超时",
					logger.String("method", info.FullMethod),
					logger.Duration("timeout", o.timeout),
				)
			}
			if o.onTimeout != nil {
				o.onTimeout(ctx, o.timeout)
			}
			return nil, status.Error(codes.DeadlineExceeded, "request timeout")

		case r := <-done:
			return r.resp, r.err
		}
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器超时拦截器.
// 注意: 流超时比较复杂，此拦截器只设置初始超时。
// 对于长时间运行的流，建议在业务逻辑中自行管理超时.
// 示例:
//
//	srv := grpc.NewServer(
//	    grpc.ChainStreamInterceptor(
//	        timeout.StreamServerInterceptor(30*time.Second),
//	    ),
//	)
func StreamServerInterceptor(timeout time.Duration, opts ...Option) grpc.StreamServerInterceptor {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(timeout, opts)

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, cancel := Cascade(ss.Context(), o.timeout)
		defer cancel()

		// 包装 ServerStream 以使用新的 context
		wrapped := grpcx.WrapServerStream(ss, ctx)

		type result struct {
			err error
		}
		done := make(chan result, 1)

		go func() {
			err := handler(srv, wrapped)
			done <- result{err: err}
		}()

		select {
		case <-ctx.Done():
			if o.logger != nil {
				logger.FromContext(ctx).Warn(
					"[Timeout] gRPC流请求超时",
					logger.String("method", info.FullMethod),
					logger.Duration("timeout", o.timeout),
				)
			}
			if o.onTimeout != nil {
				o.onTimeout(ctx, o.timeout)
			}
			return status.Error(codes.DeadlineExceeded, "stream timeout")

		case r := <-done:
			return r.err
		}
	}
}

// UnaryClientInterceptor 返回 gRPC 一元客户端超时拦截器.
// 为所有出站请求设置默认超时（如果未设置）.
// 示例:
//
//	conn, _ := grpc.Dial(target,
//	    grpc.WithUnaryInterceptor(timeout.UnaryClientInterceptor(5*time.Second)),
//	)
func UnaryClientInterceptor(timeout time.Duration, opts ...Option) grpc.UnaryClientInterceptor {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(timeout, opts)

	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		callOpts ...grpc.CallOption,
	) error {
		// 只在没有 deadline 时设置
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, o.timeout)
			defer cancel()
		}

		err := invoker(ctx, method, req, reply, cc, callOpts...)
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			if o.logger != nil {
				logger.FromContext(ctx).Warn(
					"[Timeout] gRPC客户端请求超时",
					logger.String("method", method),
					logger.Duration("timeout", o.timeout),
				)
			}
			if o.onTimeout != nil {
				o.onTimeout(ctx, o.timeout)
			}
		}
		return err
	}
}

// StreamClientInterceptor 返回 gRPC 流客户端超时拦截器.
// 为流的建立设置超时（不影响流的整体生命周期）.
func StreamClientInterceptor(timeout time.Duration, opts ...Option) grpc.StreamClientInterceptor {
	if timeout <= 0 {
		panic("timeout: 超时时间必须为正数")
	}

	o := applyOptions(timeout, opts)

	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		callOpts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		// 只在没有 deadline 时设置
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, o.timeout)
			defer cancel()
		}

		stream, err := streamer(ctx, desc, cc, method, callOpts...)
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			if o.logger != nil {
				logger.FromContext(ctx).Warn(
					"[Timeout] gRPC客户端流超时",
					logger.String("method", method),
					logger.Duration("timeout", o.timeout),
				)
			}
			if o.onTimeout != nil {
				o.onTimeout(ctx, o.timeout)
			}
		}
		return stream, err
	}
}
