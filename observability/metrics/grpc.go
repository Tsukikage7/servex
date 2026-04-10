package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor 返回 gRPC 一元服务端指标拦截器.
// 使用示例:
//
//	collector, _ := metrics.New(cfg)
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(collector)),
//	)
func UnaryServerInterceptor(collector *PrometheusCollector) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		code := "OK"
		if err != nil {
			s, _ := status.FromError(err)
			code = s.Code().String()
		}

		collector.RecordGRPCRequest(
			info.FullMethod,
			"server",
			code,
			time.Since(start),
		)

		return resp, err
	}
}

// StreamServerInterceptor 返回 gRPC 流式服务端指标拦截器.
// 使用示例:
//
//	collector, _ := metrics.New(cfg)
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(metrics.StreamServerInterceptor(collector)),
//	)
func StreamServerInterceptor(collector *PrometheusCollector) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		err := handler(srv, ss)

		code := "OK"
		if err != nil {
			s, _ := status.FromError(err)
			code = s.Code().String()
		}

		collector.RecordGRPCRequest(
			info.FullMethod,
			"server",
			code,
			time.Since(start),
		)

		return err
	}
}

// UnaryClientInterceptor 返回 gRPC 一元客户端指标拦截器.
// 使用示例:
//
//	collector, _ := metrics.New(cfg)
//	conn, _ := grpc.Dial(addr,
//	    grpc.WithUnaryInterceptor(metrics.UnaryClientInterceptor(collector)),
//	)
func UnaryClientInterceptor(collector *PrometheusCollector) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		start := time.Now()

		err := invoker(ctx, method, req, reply, cc, opts...)

		code := "OK"
		if err != nil {
			s, _ := status.FromError(err)
			code = s.Code().String()
		}

		collector.RecordGRPCRequest(
			method,
			"client",
			code,
			time.Since(start),
		)

		return err
	}
}

// StreamClientInterceptor 返回 gRPC 流式客户端指标拦截器.
// 使用示例:
//
//	collector, _ := metrics.New(cfg)
//	conn, _ := grpc.Dial(addr,
//	    grpc.WithStreamInterceptor(metrics.StreamClientInterceptor(collector)),
//	)
func StreamClientInterceptor(collector *PrometheusCollector) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		start := time.Now()

		clientStream, err := streamer(ctx, desc, cc, method, opts...)

		code := "OK"
		if err != nil {
			s, _ := status.FromError(err)
			code = s.Code().String()
		}

		collector.RecordGRPCRequest(
			method,
			"client",
			code,
			time.Since(start),
		)

		return clientStream, err
	}
}
