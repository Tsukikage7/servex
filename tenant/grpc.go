package tenant

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

// UnaryServerInterceptor 返回 gRPC 一元服务器租户拦截器.
// 默认 TokenExtractor 为 MetadataTokenExtractor("x-tenant-token").
// 示例:
//
//	srv := grpc.NewServer(
//	    grpc.UnaryInterceptor(tenant.UnaryServerInterceptor(resolver)),
//	)
func UnaryServerInterceptor(resolver Resolver, opts ...Option) grpc.UnaryServerInterceptor {
	if resolver == nil {
		panic("tenant: 解析器不能为空")
	}

	o := applyOptions(opts)

	if o.tokenExtractor == nil {
		o.tokenExtractor = MetadataTokenExtractor("x-tenant-token")
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 检查是否跳过
		if o.skipper != nil && o.skipper(ctx, req) {
			return handler(ctx, req)
		}

		// 提取令牌
		token, err := o.tokenExtractor(ctx, req)
		if err != nil {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Debug("gRPC 令牌提取失败",
					logger.String("method", info.FullMethod),
					logger.Err(err),
				)
			}
			return nil, status.Error(codes.Unauthenticated, "tenant token required")
		}

		// 解析租户
		t, err := resolver.Resolve(ctx, token)
		if err != nil {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Warn("gRPC 解析失败",
					logger.String("method", info.FullMethod),
					logger.Err(err),
				)
			}
			return nil, status.Error(codes.Unauthenticated, "invalid tenant")
		}

		// 检查租户是否启用
		if !t.TenantEnabled() {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Warn("gRPC 租户已禁用",
					logger.String("tenant_id", t.TenantID()),
					logger.String("method", info.FullMethod),
				)
			}
			return nil, status.Error(codes.PermissionDenied, "tenant disabled")
		}

		ctx = WithTenant(ctx, t)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器租户拦截器.
func StreamServerInterceptor(resolver Resolver, opts ...Option) grpc.StreamServerInterceptor {
	if resolver == nil {
		panic("tenant: 解析器不能为空")
	}

	o := applyOptions(opts)

	if o.tokenExtractor == nil {
		o.tokenExtractor = MetadataTokenExtractor("x-tenant-token")
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// 检查是否跳过
		if o.skipper != nil && o.skipper(ctx, nil) {
			return handler(srv, ss)
		}

		// 提取令牌
		token, err := o.tokenExtractor(ctx, nil)
		if err != nil {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Debug("gRPC 流令牌提取失败",
					logger.String("method", info.FullMethod),
					logger.Err(err),
				)
			}
			return status.Error(codes.Unauthenticated, "tenant token required")
		}

		// 解析租户
		t, err := resolver.Resolve(ctx, token)
		if err != nil {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Warn("gRPC 流解析失败",
					logger.String("method", info.FullMethod),
					logger.Err(err),
				)
			}
			return status.Error(codes.Unauthenticated, "invalid tenant")
		}

		// 检查租户是否启用
		if !t.TenantEnabled() {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Warn("gRPC 流租户已禁用",
					logger.String("tenant_id", t.TenantID()),
					logger.String("method", info.FullMethod),
				)
			}
			return status.Error(codes.PermissionDenied, "tenant disabled")
		}

		ctx = WithTenant(ctx, t)
		return handler(srv, grpcx.WrapServerStream(ss, ctx))
	}
}
