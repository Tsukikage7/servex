package semaphore

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// MiddlewareOption 中间件配置选项.
type MiddlewareOption func(*middlewareOptions)

type middlewareOptions struct {
	logger logger.Logger
	block  bool // 是否阻塞等待，默认 false（直接拒绝）
}

func defaultMiddlewareOptions() *middlewareOptions {
	return &middlewareOptions{
		block: false,
	}
}

// WithMiddlewareLogger 设置日志记录器.
func WithMiddlewareLogger(log logger.Logger) MiddlewareOption {
	return func(o *middlewareOptions) {
		o.logger = log
	}
}

// WithBlock 设置是否阻塞等待.
// 如果为 true，当没有可用许可时会阻塞等待.
// 如果为 false（默认），会立即返回错误.
func WithBlock(block bool) MiddlewareOption {
	return func(o *middlewareOptions) {
		o.block = block
	}
}

// EndpointMiddleware 返回 Endpoint 信号量中间件.
// 限制 endpoint 的并发调用数量.
// 示例:
//
//	sem := semaphore.NewLocal(10)
//	endpoint = semaphore.EndpointMiddleware(sem)(endpoint)
func EndpointMiddleware(sem Semaphore, opts ...MiddlewareOption) endpoint.Middleware {
	o := defaultMiddlewareOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			var acquired bool

			if o.block {
				if err := sem.Acquire(ctx); err != nil {
					if o.logger != nil {
						logger.FromContextOr(ctx, o.logger).Warn(
							"[Semaphore] 获取许可失败",
							logger.Err(err),
						)
					}
					return nil, ErrNoPermit
				}
				acquired = true
			} else {
				acquired = sem.TryAcquire(ctx)
				if !acquired {
					if o.logger != nil {
						logger.FromContextOr(ctx, o.logger).Debug("[Semaphore] 无可用许可")
					}
					return nil, ErrNoPermit
				}
			}

			defer func() {
				if acquired {
					_ = sem.Release(ctx)
				}
			}()

			return next(ctx, request)
		}
	}
}

// HTTPMiddleware 返回 HTTP 信号量中间件.
// 限制 HTTP 请求的并发处理数量.
// 示例:
//
//	sem := semaphore.NewLocal(100)
//	handler = semaphore.HTTPMiddleware(sem)(handler)
func HTTPMiddleware(sem Semaphore, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	o := defaultMiddlewareOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			var acquired bool

			if o.block {
				if err := sem.Acquire(ctx); err != nil {
					if o.logger != nil {
						logger.FromContextOr(ctx, o.logger).Warn(
							"[Semaphore] 获取许可失败",
							logger.String("method", r.Method),
							logger.String("path", r.URL.Path),
							logger.Err(err),
						)
					}
					http.Error(w, "Service Unavailable: too many concurrent requests", http.StatusServiceUnavailable)
					return
				}
				acquired = true
			} else {
				acquired = sem.TryAcquire(ctx)
				if !acquired {
					if o.logger != nil {
						logger.FromContextOr(ctx, o.logger).Debug(
							"[Semaphore] 无可用许可",
							logger.String("method", r.Method),
							logger.String("path", r.URL.Path),
						)
					}
					http.Error(w, "Service Unavailable: too many concurrent requests", http.StatusServiceUnavailable)
					return
				}
			}

			defer func() {
				if acquired {
					_ = sem.Release(ctx)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// UnaryServerInterceptor 返回 gRPC 一元服务器信号量拦截器.
// 限制 gRPC 请求的并发处理数量.
// 示例:
//
//	sem := semaphore.NewLocal(100)
//	srv := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        semaphore.UnaryServerInterceptor(sem),
//	    ),
//	)
func UnaryServerInterceptor(sem Semaphore, opts ...MiddlewareOption) grpc.UnaryServerInterceptor {
	o := defaultMiddlewareOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var acquired bool

		if o.block {
			if err := sem.Acquire(ctx); err != nil {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Warn(
						"[Semaphore] 获取许可失败",
						logger.String("method", info.FullMethod),
						logger.Err(err),
					)
				}
				return nil, status.Error(codes.ResourceExhausted, "too many concurrent requests")
			}
			acquired = true
		} else {
			acquired = sem.TryAcquire(ctx)
			if !acquired {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Debug(
						"[Semaphore] 无可用许可",
						logger.String("method", info.FullMethod),
					)
				}
				return nil, status.Error(codes.ResourceExhausted, "too many concurrent requests")
			}
		}

		defer func() {
			if acquired {
				_ = sem.Release(ctx)
			}
		}()

		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器信号量拦截器.
// 限制 gRPC 流的并发处理数量.
func StreamServerInterceptor(sem Semaphore, opts ...MiddlewareOption) grpc.StreamServerInterceptor {
	o := defaultMiddlewareOptions()
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
		var acquired bool

		if o.block {
			if err := sem.Acquire(ctx); err != nil {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Warn(
						"[Semaphore] 获取许可失败",
						logger.String("method", info.FullMethod),
						logger.Err(err),
					)
				}
				return status.Error(codes.ResourceExhausted, "too many concurrent requests")
			}
			acquired = true
		} else {
			acquired = sem.TryAcquire(ctx)
			if !acquired {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Debug(
						"[Semaphore] 无可用许可",
						logger.String("method", info.FullMethod),
					)
				}
				return status.Error(codes.ResourceExhausted, "too many concurrent requests")
			}
		}

		defer func() {
			if acquired {
				_ = sem.Release(ctx)
			}
		}()

		return handler(srv, ss)
	}
}
