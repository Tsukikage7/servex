package retry

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCRetryableFunc 判断 gRPC 错误是否应该重试.
type GRPCRetryableFunc func(err error) bool

// UnaryClientInterceptor 返回 gRPC 一元客户端重试拦截器.
// 使用示例:
//
//	cfg := retry.DefaultConfig()
//	conn, _ := grpc.Dial("localhost:50051",
//	    grpc.WithUnaryInterceptor(retry.UnaryClientInterceptor(cfg)),
//	)
func UnaryClientInterceptor(cfg *Config) grpc.UnaryClientInterceptor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Backoff == nil {
		cfg.Backoff = FixedBackoff
	}

	retryable := DefaultGRPCRetryable
	if cfg.Retryable != nil {
		retryable = cfg.Retryable
	}

	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var err error

		for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// 执行 RPC 调用
			err = invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				return nil
			}

			// 判断是否应该重试
			if !retryable(err) {
				return err
			}

			// 如果不是最后一次尝试，则等待
			if attempt < cfg.MaxAttempts-1 {
				wait := cfg.Backoff(attempt, cfg.Delay)
				retryTimer := time.NewTimer(wait)
				select {
				case <-retryTimer.C:
					continue
				case <-ctx.Done():
					retryTimer.Stop()
					return ctx.Err()
				}
			}
		}

		return err
	}
}

// StreamClientInterceptor 返回 gRPC 流式客户端重试拦截器.
// 注意：流式 RPC 的重试只在建立连接阶段生效。
// 一旦流开始传输，重试可能导致数据不一致。
// 使用示例:
//
//	cfg := retry.DefaultConfig()
//	conn, _ := grpc.Dial("localhost:50051",
//	    grpc.WithStreamInterceptor(retry.StreamClientInterceptor(cfg)),
//	)
func StreamClientInterceptor(cfg *Config) grpc.StreamClientInterceptor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Backoff == nil {
		cfg.Backoff = FixedBackoff
	}

	retryable := DefaultGRPCRetryable
	if cfg.Retryable != nil {
		retryable = cfg.Retryable
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		var stream grpc.ClientStream
		var err error

		for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			// 创建流
			stream, err = streamer(ctx, desc, cc, method, opts...)
			if err == nil {
				return stream, nil
			}

			// 判断是否应该重试
			if !retryable(err) {
				return nil, err
			}

			// 如果不是最后一次尝试，则等待
			if attempt < cfg.MaxAttempts-1 {
				wait := cfg.Backoff(attempt, cfg.Delay)
				retryTimer := time.NewTimer(wait)
				select {
				case <-retryTimer.C:
					continue
				case <-ctx.Done():
					retryTimer.Stop()
					return nil, ctx.Err()
				}
			}
		}

		return stream, err
	}
}

// DefaultGRPCRetryable 默认的 gRPC 重试判断.
// 重试临时错误和资源耗尽错误.
func DefaultGRPCRetryable(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return true // 非 gRPC 错误，重试
	}

	switch st.Code() {
	case codes.Unavailable: // 服务不可用
		return true
	case codes.ResourceExhausted: // 资源耗尽（如限流）
		return true
	case codes.Aborted: // 操作中止（通常是并发问题）
		return true
	case codes.DeadlineExceeded: // 超时
		return true
	default:
		return false
	}
}

// RetryableCodesFunc 返回根据状态码判断是否重试的函数.
func RetryableCodesFunc(retryCodes ...codes.Code) RetryableFunc {
	codeSet := make(map[codes.Code]struct{}, len(retryCodes))
	for _, code := range retryCodes {
		codeSet[code] = struct{}{}
	}

	return func(err error) bool {
		if err == nil {
			return false
		}

		st, ok := status.FromError(err)
		if !ok {
			return false
		}

		_, shouldRetry := codeSet[st.Code()]
		return shouldRetry
	}
}

// GRPCRetrier 提供可配置的 gRPC 重试器.
type GRPCRetrier struct {
	cfg *Config
}

// NewGRPCRetrier 创建 gRPC 重试器.
func NewGRPCRetrier(cfg *Config) *GRPCRetrier {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &GRPCRetrier{cfg: cfg}
}

// UnaryClientInterceptor 返回一元客户端拦截器.
func (r *GRPCRetrier) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return UnaryClientInterceptor(r.cfg)
}

// StreamClientInterceptor 返回流式客户端拦截器.
func (r *GRPCRetrier) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return StreamClientInterceptor(r.cfg)
}

// WithMaxAttempts 设置最大重试次数.
func (r *GRPCRetrier) WithMaxAttempts(n int) *GRPCRetrier {
	r.cfg.MaxAttempts = n
	return r
}

// WithDelay 设置重试间隔.
func (r *GRPCRetrier) WithDelay(d time.Duration) *GRPCRetrier {
	r.cfg.Delay = d
	return r
}

// WithBackoff 设置退避策略.
func (r *GRPCRetrier) WithBackoff(fn BackoffFunc) *GRPCRetrier {
	r.cfg.Backoff = fn
	return r
}

// WithRetryableCodes 设置可重试的状态码.
func (r *GRPCRetrier) WithRetryableCodes(codes ...codes.Code) *GRPCRetrier {
	r.cfg.Retryable = RetryableCodesFunc(codes...)
	return r
}
