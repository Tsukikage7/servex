package grpcx

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/Tsukikage7/servex/v2/errors"
)

var (
	// ErrHealthCheckFailed 健康检查请求失败.
	ErrHealthCheckFailed = errors.New(60150, "transport.grpcx.health_check_failed", "健康检查失败")
	// ErrServiceNotServing 服务状态异常.
	ErrServiceNotServing = errors.New(60151, "transport.grpcx.service_not_serving", "服务状态异常")
	// ErrWaitForReadyTimeout 等待连接就绪超时.
	ErrWaitForReadyTimeout = errors.New(60152, "transport.grpcx.wait_for_ready_timeout", "等待连接就绪超时")
)

// HealthCheck 创建 gRPC 健康检查客户端，检查目标服务是否可用.
// 发送标准 gRPC 健康检查请求，验证服务状态是否为 SERVING.
func HealthCheck(ctx context.Context, conn *grpc.ClientConn) error {
	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return ErrHealthCheckFailed.WithCause(err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return ErrServiceNotServing.WithMessage(
			fmt.Sprintf("服务状态异常: %s", resp.GetStatus().String()),
		)
	}
	return nil
}

// WaitForReady 等待 gRPC 连接就绪.
// 在指定超时时间内等待连接状态变为 READY，超时则返回错误.
func WaitForReady(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if !conn.WaitForStateChange(ctx, state) {
			// context 超时或取消
			return ErrWaitForReadyTimeout.WithMessage(
				fmt.Sprintf("等待连接就绪超时，当前状态: %s", conn.GetState().String()),
			)
		}
	}
}
