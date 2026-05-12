// Package grpcclient 提供 gRPC 客户端工具.
package grpcclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
	"github.com/Tsukikage7/servex/v2/observability/tracing"
)

// Client gRPC 客户端封装.
type Client struct {
	conn *grpc.ClientConn
	opts *options
}

// New 创建 gRPC 客户端，必需设置 serviceName 和 discovery.
func New(opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// 验证必需参数
	if o.serviceName == "" {
		return nil, ErrMissingServiceName
	}
	if o.discovery == nil {
		return nil, ErrMissingDiscovery
	}
	if o.logger == nil {
		o.logger = logger.Nop()
	}
	o.logger = logger.WithComponent(o.logger, "gRPC")

	// 服务发现
	addrs, err := o.discovery.Discover(context.Background(), o.serviceName)
	if err != nil {
		return nil, ErrDiscoveryFailed.WithCause(err)
	}
	if len(addrs) == 0 {
		return nil, ErrServiceNotFound.WithMessage(o.serviceName)
	}
	target := addrs[0]

	dialOpts := buildDialOptions(o)

	// 创建连接
	conn, err := dialWithTimeout(target, o.timeout, dialOpts...)
	if err != nil {
		return nil, ErrConnectionFailed.WithCause(err)
	}

	o.logger.With(
		logger.String("name", o.name),
		logger.String("service", o.serviceName),
		logger.String("target", target),
	).Info("客户端初始化成功")

	return &Client{
		conn: conn,
		opts: o,
	}, nil
}

// buildDialOptions 从 options 构建 grpc.DialOption 切片.
func buildDialOptions(o *options) []grpc.DialOption {
	var dialOpts []grpc.DialOption

	// TLS credentials
	if o.tlsConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(o.tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Keepalive
	dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                o.keepaliveTime,
		Timeout:             o.keepaliveTimeout,
		PermitWithoutStream: true,
	}))

	// WaitForReady
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(
		grpc.WaitForReady(true),
	))

	// Load balancer
	if o.balancerPolicy != "" {
		svcCfg := fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, o.balancerPolicy)
		dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(svcCfg))
	}

	// Build unary interceptor chain
	var unaryInterceptors []grpc.UnaryClientInterceptor

	// Logging interceptor
	if o.enableLogging && o.logger != nil {
		unaryInterceptors = append(unaryInterceptors, loggingUnaryInterceptor(o.logger))
	}

	// Retry interceptor
	if o.retryMaxAttempts > 1 {
		unaryInterceptors = append(unaryInterceptors, retryUnaryInterceptor(o.retryMaxAttempts, o.retryBackoff))
	}

	// Circuit breaker interceptor
	if o.circuitBreaker != nil {
		unaryInterceptors = append(unaryInterceptors, circuitBreakerUnaryInterceptor(o.circuitBreaker))
	}

	// Tracing interceptors
	if o.tracerName != "" {
		unaryInterceptors = append(unaryInterceptors, tracing.UnaryClientInterceptor(o.tracerName))
	}

	// Metrics interceptors
	if o.metricsCollector != nil {
		unaryInterceptors = append(unaryInterceptors, metrics.UnaryClientInterceptor(o.metricsCollector))
	}

	// Custom interceptors
	unaryInterceptors = append(unaryInterceptors, o.interceptors...)

	if len(unaryInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(unaryInterceptors...))
	}

	// Build stream interceptor chain
	var streamInterceptors []grpc.StreamClientInterceptor

	// Tracing stream interceptor
	if o.tracerName != "" {
		streamInterceptors = append(streamInterceptors, tracing.StreamClientInterceptor(o.tracerName))
	}

	// Metrics stream interceptor
	if o.metricsCollector != nil {
		streamInterceptors = append(streamInterceptors, metrics.StreamClientInterceptor(o.metricsCollector))
	}

	// Custom stream interceptors
	streamInterceptors = append(streamInterceptors, o.streamInterceptors...)

	if len(streamInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(streamInterceptors...))
	}

	// Additional dial options
	dialOpts = append(dialOpts, o.dialOptions...)

	return dialOpts
}

// dialWithTimeout 使用可选超时创建 gRPC 连接.
func dialWithTimeout(target string, timeout time.Duration, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if timeout > 0 {
		dialOpts = append(dialOpts, grpc.WithConnectParams(grpc.ConnectParams{
			MinConnectTimeout: timeout,
		}))
	}
	return grpc.NewClient(target, dialOpts...)
}

// loggingUnaryInterceptor 返回记录方法调用、耗时和错误的一元拦截器.
func loggingUnaryInterceptor(log logger.Logger) grpc.UnaryClientInterceptor {
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
		elapsed := time.Since(start)

		if err != nil {
			log.With(
				logger.String("method", method),
				logger.Duration("elapsed", elapsed),
				logger.Err(err),
			).Error("调用失败")
		} else {
			log.With(
				logger.String("method", method),
				logger.Duration("elapsed", elapsed),
			).Debug("调用完成")
		}
		return err
	}
}

// retryUnaryInterceptor 返回简单重试一元拦截器.
//
// 仅在 Unavailable 或 DeadlineExceeded 错误码时重试.
func retryUnaryInterceptor(maxAttempts int, backoff time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		var lastErr error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			lastErr = invoker(ctx, method, req, reply, cc, opts...)
			if lastErr == nil {
				return nil
			}

			// 仅对 Unavailable 和 DeadlineExceeded 重试
			st, ok := status.FromError(lastErr)
			if !ok || (st.Code() != codes.Unavailable && st.Code() != codes.DeadlineExceeded) {
				return lastErr
			}

			// 最后一次不等待
			if attempt < maxAttempts-1 {
				retryTimer := time.NewTimer(backoff)
				select {
				case <-retryTimer.C:
				case <-ctx.Done():
					retryTimer.Stop()
					return ctx.Err()
				}
			}
		}
		return lastErr
	}
}

// circuitBreakerUnaryInterceptor 返回熔断器一元拦截器.
func circuitBreakerUnaryInterceptor(cb interface {
	Execute(context.Context, func() error) error
}) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return cb.Execute(ctx, func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}

// Conn 返回底层 gRPC 连接.
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// Close 关闭连接.
func (c *Client) Close() error {
	if c.conn != nil {
		if c.opts.logger != nil {
			c.opts.logger.With(
				logger.String("name", c.opts.name),
				logger.String("service", c.opts.serviceName),
			).Info("关闭连接")
		}
		return c.conn.Close()
	}
	return nil
}
