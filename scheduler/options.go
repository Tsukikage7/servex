package scheduler

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/storage/lock"
)

// Option 调度器配置选项.
type Option func(*options)

// options 调度器内部配置.
type options struct {
	logger         logger.Logger
	locker         lock.Locker
	hooks          *Hooks
	defaultTimeout time.Duration
	lockTTL        time.Duration
	withSeconds    bool
	location       *time.Location
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		defaultTimeout: 5 * time.Minute,
		lockTTL:        10 * time.Minute,
		withSeconds:    true,
		location:       time.Local,
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

// WithLocker 设置分布式锁.
// 用于分布式任务调度，确保同一任务在多实例间只执行一次.
// 需要配合 Job.Distributed 使用.
// 示例:
//
//	redisCache, _ := cache.New(&cache.Config{Type: "redis", ...})
//	locker := lock.NewRedis(redisCache, lock.WithKeyPrefix("scheduler:"))
//	s := scheduler.New(scheduler.WithLocker(locker))
func WithLocker(l lock.Locker) Option {
	return func(o *options) {
		o.locker = l
	}
}

// WithHooks 设置全局钩子.
// 对所有任务生效.
func WithHooks(hooks *Hooks) Option {
	return func(o *options) {
		o.hooks = hooks
	}
}

// WithDefaultTimeout 设置默认任务超时时间.
// 如果任务未指定超时时间，将使用此值.
// 默认: 5 分钟.
func WithDefaultTimeout(d time.Duration) Option {
	return func(o *options) {
		o.defaultTimeout = d
	}
}

// WithLockTTL 设置分布式锁默认过期时间.
// 应大于任务最大执行时间.
// 默认: 10 分钟.
func WithLockTTL(d time.Duration) Option {
	return func(o *options) {
		o.lockTTL = d
	}
}

// WithSeconds 设置是否支持秒级调度.
// 启用后，Cron 表达式格式为: 秒 分 时 日 月 周
// 禁用后，Cron 表达式格式为: 分 时 日 月 周
// 默认: 启用.
func WithSeconds(enabled bool) Option {
	return func(o *options) {
		o.withSeconds = enabled
	}
}

// WithLocation 设置时区.
// 默认: time.Local
func WithLocation(loc *time.Location) Option {
	return func(o *options) {
		o.location = loc
	}
}
