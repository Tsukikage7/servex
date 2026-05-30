// Package timeout 提供统一的超时控制中间件.
// 支持 Endpoint、HTTP 和 gRPC 三种级别的超时控制，
// 并支持级联超时调用下游时自动减去已用时间。
// 基本用法:
//
//	// Endpoint 中间件
//	endpoint = timeout.EndpointMiddleware(5*time.Second)(endpoint)
//	// HTTP 中间件
//	handler = timeout.HTTPMiddleware(10*time.Second)(handler)
//	// gRPC 拦截器
//	grpc.NewServer(
//	    grpc.UnaryInterceptor(timeout.UnaryServerInterceptor(5*time.Second)),
//	)
//
// 级联超时:
//
//	// 自动计算剩余时间
//	ctx, cancel := timeout.Cascade(ctx, 2*time.Second)
//	defer cancel()
package timeout

import (
	"context"
	"time"
)

// Remaining 返回 context 中的剩余超时时间.
// 如果 context 没有设置 deadline，返回 0 和 false.
// 如果 deadline 已过，返回负值和 true.
func Remaining(ctx context.Context) (time.Duration, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(deadline), true
}

// Cascade 创建级联超时 context.
// 如果父 context 有 deadline，则取 min(父剩余时间, timeout) 作为新 deadline.
// 如果父 context 没有 deadline，则直接使用 timeout.
// 示例:
//
//	// 调用下游服务时，自动减去已用时间
//	ctx, cancel := timeout.Cascade(ctx, 2*time.Second)
//	defer cancel()
//	resp, err := downstreamClient.Call(ctx, req)
func Cascade(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}

	remaining, hasDeadline := Remaining(ctx)
	if hasDeadline && remaining < timeout {
		// 父 context 剩余时间更短，使用父 context 的 deadline
		return context.WithTimeout(ctx, remaining)
	}

	return context.WithTimeout(ctx, timeout)
}

// ShrinkBy 创建一个减去指定时间的超时 context.
// 用于预留处理时间，确保有足够时间处理超时后的清理工作.
// 示例:
//
//	// 预留 500ms 处理超时响应
//	ctx, cancel := timeout.ShrinkBy(ctx, 500*time.Millisecond)
//	defer cancel()
func ShrinkBy(ctx context.Context, buffer time.Duration) (context.Context, context.CancelFunc) {
	remaining, hasDeadline := Remaining(ctx)
	if !hasDeadline {
		return ctx, func() {}
	}

	newTimeout := remaining - buffer
	if newTimeout <= 0 {
		// 已经没有足够时间，立即取消
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		return ctx, cancel
	}

	return context.WithTimeout(ctx, newTimeout)
}

// WithTimeout 创建带超时的 context.
// 这是 context.WithTimeout 的便捷包装，添加了参数校验.
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if timeout <= 0 {
		return nil, nil, ErrInvalidTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return ctx, cancel, nil
}
