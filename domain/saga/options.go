package saga

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Option 配置选项函数.
type Option func(*options)

// options Saga 配置.
type options struct {
	store       Store
	logger      logger.Logger
	timeout     time.Duration
	retryCount  int
	retryDelay  time.Duration
	onStepStart func(stepName string)
	onStepEnd   func(stepName string, err error)
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		store:      newNopStore(),
		timeout:    0, // 无超时
		retryCount: 0, // 不重试
		retryDelay: time.Second,
	}
}

// applyOptions 应用配置选项.
func applyOptions(opts []Option) *options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithStore 设置状态存储.
// 如果不设置，使用内部空存储不保存状态.
// 生产环境建议使用 NewRedisStore 保存状态.
func WithStore(store Store) Option {
	return func(o *options) {
		o.store = store
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = logger.WithComponent(log, "Saga")
	}
}

// WithTimeout 设置执行超时时间.
// 超时后会停止执行并开始补偿.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// WithRetry 设置重试配置.
// count: 重试次数
// delay: 重试间隔
func WithRetry(count int, delay time.Duration) Option {
	return func(o *options) {
		o.retryCount = count
		o.retryDelay = delay
	}
}

// WithStepHooks 设置步骤执行钩子.
func WithStepHooks(onStart func(stepName string), onEnd func(stepName string, err error)) Option {
	return func(o *options) {
		o.onStepStart = onStart
		o.onStepEnd = onEnd
	}
}
