package activity

import (
	"context"
	"math/rand"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/httpx/clientip"
)

// UnaryServerInterceptor 返回一元 gRPC 拦截器，自动追踪用户活跃.
func UnaryServerInterceptor(tracker *Tracker, opts ...GRPCInterceptorOption) grpc.UnaryServerInterceptor {
	o := defaultGRPCOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 跳过指定方法
		if shouldSkipMethod(info.FullMethod, o.skipMethods) {
			return handler(ctx, req)
		}

		// 采样
		if tracker.opts.sampleRate < 1.0 && rand.Float64() > tracker.opts.sampleRate {
			return handler(ctx, req)
		}

		// 提取用户 ID
		var userID string
		if tracker.opts.extractor != nil {
			userID = tracker.opts.extractor(ctx)
		}

		// 追踪活跃
		if userID != "" {
			event := &Event{
				UserID:    userID,
				EventType: EventTypeRequest,
				Path:      info.FullMethod,
				IP:        clientip.GetIP(ctx),
				Platform:  getPlatformFromMetadata(ctx),
			}
			_ = tracker.Track(ctx, event)
		}

		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回流 gRPC 拦截器，自动追踪用户活跃.
func StreamServerInterceptor(tracker *Tracker, opts ...GRPCInterceptorOption) grpc.StreamServerInterceptor {
	o := defaultGRPCOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// 跳过指定方法
		if shouldSkipMethod(info.FullMethod, o.skipMethods) {
			return handler(srv, ss)
		}

		// 采样
		if tracker.opts.sampleRate < 1.0 && rand.Float64() > tracker.opts.sampleRate {
			return handler(srv, ss)
		}

		// 提取用户 ID
		var userID string
		if tracker.opts.extractor != nil {
			userID = tracker.opts.extractor(ctx)
		}

		// 追踪活跃
		if userID != "" {
			event := &Event{
				UserID:    userID,
				EventType: EventTypeRequest,
				Path:      info.FullMethod,
				IP:        clientip.GetIP(ctx),
				Platform:  getPlatformFromMetadata(ctx),
			}
			_ = tracker.Track(ctx, event)
		}

		return handler(srv, ss)
	}
}

// GRPCInterceptorOption gRPC 拦截器配置选项.
type GRPCInterceptorOption func(*grpcInterceptorOptions)

type grpcInterceptorOptions struct {
	skipMethods map[string]bool
}

func defaultGRPCOptions() *grpcInterceptorOptions {
	return &grpcInterceptorOptions{
		skipMethods: map[string]bool{
			"/grpc.health.v1.Health/Check": true,
			"/grpc.health.v1.Health/Watch": true,
		},
	}
}

// WithSkipMethods 设置跳过的方法.
func WithSkipMethods(methods ...string) GRPCInterceptorOption {
	return func(o *grpcInterceptorOptions) {
		for _, m := range methods {
			o.skipMethods[m] = true
		}
	}
}

// shouldSkipMethod 检查是否应跳过该方法.
func shouldSkipMethod(method string, skipMethods map[string]bool) bool {
	if skipMethods[method] {
		return true
	}
	// 检查前缀匹配
	for m := range skipMethods {
		if strings.HasSuffix(m, "*") {
			prefix := m[:len(m)-1]
			if strings.HasPrefix(method, prefix) {
				return true
			}
		}
	}
	return false
}

// getPlatformFromMetadata 从 gRPC metadata 获取平台信息.
func getPlatformFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "Unknown"
	}

	// 尝试从自定义 header 获取
	if values := md.Get("x-platform"); len(values) > 0 {
		return values[0]
	}

	// 从 Sec-CH-UA-Platform 获取
	if values := md.Get("sec-ch-ua-platform"); len(values) > 0 {
		return trimQuotes(values[0])
	}

	return "Unknown"
}
