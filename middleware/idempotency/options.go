package idempotency

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Option 配置选项函数.
type Option func(*options)

// options 幂等性配置.
type options struct {
	store        Store
	keyExtractor KeyExtractor
	ttl          time.Duration
	logger       logger.Logger
	skipOnError  bool // 存储错误时是否跳过幂等检查
	lockTimeout  time.Duration
}

// defaultOptions 返回默认配置.
func defaultOptions(store Store) *options {
	return &options{
		store:       store,
		ttl:         DefaultTTL,
		skipOnError: false,
		lockTimeout: 30 * time.Second,
	}
}

// applyOptions 应用配置选项.
func applyOptions(store Store, opts []Option) *options {
	o := defaultOptions(store)
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithKeyExtractor 设置幂等键提取函数.
// 默认行为:
//   - HTTP: 从 Idempotency-Key 请求头提取
//   - gRPC: 从 x-idempotency-key 元数据提取
//   - Endpoint: 调用 request.(IdempotentRequest).IdempotencyKey()
func WithKeyExtractor(fn KeyExtractor) Option {
	return func(o *options) {
		o.keyExtractor = fn
	}
}

// WithTTL 设置幂等键过期时间.
// 默认 24 小时.
func WithTTL(ttl time.Duration) Option {
	return func(o *options) {
		o.ttl = ttl
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

// WithSkipOnError 设置存储错误时是否跳过幂等检查.
// 如果设置为 true，当存储操作失败时，会跳过幂等检查继续处理请求.
// 默认为 false，存储失败时返回错误.
func WithSkipOnError(skip bool) Option {
	return func(o *options) {
		o.skipOnError = skip
	}
}

// WithLockTimeout 设置锁超时时间.
// 当请求正在处理中时，新请求会等待这个时间.
// 默认 30 秒.
func WithLockTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.lockTimeout = timeout
	}
}
