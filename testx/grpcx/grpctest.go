// Package grpcx 提供 gRPC 测试辅助工具.
package grpcx

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// bufSize bufconn 监听器的缓冲区大小.
const bufSize = 1024 * 1024

// NewGRPCTestServer 创建基于内存连接的 gRPC 测试服务器.
// registerFn 用于注册 gRPC 服务；interceptors 为可选的一元拦截器.
// 返回客户端连接和清理函数.
func NewGRPCTestServer(registerFn func(*grpc.Server), interceptors ...grpc.UnaryServerInterceptor) (*grpc.ClientConn, func()) {
	lis := bufconn.Listen(bufSize)

	var opts []grpc.ServerOption
	if len(interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	srv := grpc.NewServer(opts...)
	registerFn(srv)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// 服务器关闭时忽略错误.
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic("testx: 创建 gRPC 客户端连接失败: " + err.Error())
	}

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
		lis.Close()
	}
	return conn, cleanup
}
