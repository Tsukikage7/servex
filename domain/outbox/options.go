package outbox

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Option 配置选项函数.
type Option func(*options)

// options Relay 配置.
type options struct {
	logger          logger.Logger
	pollInterval    time.Duration
	batchSize       int
	maxRetries      int
	cleanupAge      time.Duration
	cleanupInterval time.Duration
	staleTimeout    time.Duration
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		pollInterval:    time.Second,
		batchSize:       100,
		maxRetries:      3,
		cleanupAge:      7 * 24 * time.Hour,
		cleanupInterval: time.Hour,
		staleTimeout:    5 * time.Minute,
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

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

// WithPollInterval 设置轮询间隔.
// 默认 1 秒.
func WithPollInterval(d time.Duration) Option {
	return func(o *options) {
		o.pollInterval = d
	}
}

// WithBatchSize 设置每次拉取的消息条数.
// 默认 100.
func WithBatchSize(size int) Option {
	return func(o *options) {
		o.batchSize = size
	}
}

// WithMaxRetries 设置最大重试次数.
// 超过此次数的失败消息不再被 ResetStale 重置.
// 默认 3.
func WithMaxRetries(n int) Option {
	return func(o *options) {
		o.maxRetries = n
	}
}

// WithCleanupAge 设置已发送消息的保留时长.
// 超过此时间的已发送消息将被清理.
// 默认 7 天.
func WithCleanupAge(d time.Duration) Option {
	return func(o *options) {
		o.cleanupAge = d
	}
}

// WithCleanupInterval 设置清理任务的执行间隔.
// 默认 1 小时.
func WithCleanupInterval(d time.Duration) Option {
	return func(o *options) {
		o.cleanupInterval = d
	}
}

// WithStaleTimeout 设置 Processing 状态的超时阈值.
// 超时的 Processing/Failed 消息将被重置为 Pending.
// 默认 5 分钟.
func WithStaleTimeout(d time.Duration) Option {
	return func(o *options) {
		o.staleTimeout = d
	}
}
