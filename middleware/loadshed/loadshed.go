// Package loadshed 提供负载卸载中间件.
// 基于并发请求数、队列深度和延迟指标，在系统过载时主动拒绝请求，
// 防止级联故障，保护服务可用性.
package loadshed

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Options 配置选项.
type Options struct {
	// MaxConcurrent 最大并发请求数.
	// 超过此值的请求将被拒绝. 0 表示不限制.
	MaxConcurrent int64

	// MaxQueueDepth 最大排队深度.
	// 当等待中的请求数超过此值时拒绝新请求. 0 表示不限制.
	MaxQueueDepth int64

	// MaxLatency 最大延迟阈值.
	// 当请求处理延迟超过此值时，开始拒绝新请求. 0 表示不限制.
	MaxLatency time.Duration

	// Logger 日志记录器.
	Logger logger.Logger
}

// Option 是配置函数.
type Option func(*Options)

// WithMaxConcurrent 设置最大并发请求数.
func WithMaxConcurrent(n int64) Option {
	return func(o *Options) {
		o.MaxConcurrent = n
	}
}

// WithMaxQueueDepth 设置最大排队深度.
func WithMaxQueueDepth(n int64) Option {
	return func(o *Options) {
		o.MaxQueueDepth = n
	}
}

// WithMaxLatency 设置最大延迟阈值.
func WithMaxLatency(d time.Duration) Option {
	return func(o *Options) {
		o.MaxLatency = d
	}
}

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = logger.WithComponent(l, "Loadshed")
	}
}

// defaultOptions 返回默认配置.
func defaultOptions() *Options {
	return &Options{}
}

// applyOptions 应用配置选项.
func applyOptions(opts []Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// shedder 负载卸载器状态.
type shedder struct {
	// inflight 当前正在处理的请求数.
	inflight atomic.Int64

	// waiting 当前等待中的请求数.
	waiting atomic.Int64

	// lastLatency 最近一次请求的处理延迟（纳秒）.
	lastLatency atomic.Int64
}

// HTTPMiddleware 返回 HTTP 负载卸载中间件.
// 当系统过载时返回 503 Service Unavailable.
// 检查顺序：并发数 > 队列深度 > 延迟.
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := applyOptions(opts)
	s := &shedder{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查排队深度
			if o.MaxQueueDepth > 0 {
				queueDepth := s.waiting.Add(1)
				defer s.waiting.Add(-1)

				if queueDepth > o.MaxQueueDepth {
					shed(w, o, "队列深度超过阈值")
					return
				}
			}

			// 检查并发数
			if o.MaxConcurrent > 0 {
				current := s.inflight.Add(1)
				defer s.inflight.Add(-1)

				if current > o.MaxConcurrent {
					shed(w, o, "并发请求数超过阈值")
					return
				}
			}

			// 检查延迟
			if o.MaxLatency > 0 {
				lastLatency := time.Duration(s.lastLatency.Load())
				if lastLatency > o.MaxLatency {
					shed(w, o, "请求延迟超过阈值")
					return
				}
			}

			// 处理请求并记录延迟
			start := time.Now()
			next.ServeHTTP(w, r)
			elapsed := time.Since(start)
			s.lastLatency.Store(int64(elapsed))
		})
	}
}

// shed 执行负载卸载：记录日志并返回 503.
func shed(w http.ResponseWriter, o *Options, reason string) {
	if o.Logger != nil {
		o.Logger.Warn("拒绝请求",
			logger.String("reason", reason),
		)
	}
	http.Error(w, "服务过载，请稍后重试", http.StatusServiceUnavailable)
}

// Stats 暴露负载卸载器的运行时状态，用于监控.
type Stats struct {
	// Inflight 当前正在处理的请求数.
	Inflight int64

	// Waiting 当前等待中的请求数.
	Waiting int64

	// LastLatency 最近一次请求的处理延迟.
	LastLatency time.Duration
}
