package app

import (
	"context"
	"os"
	"time"

	"github.com/Tsukikage7/servex/observability/logger"
)

// CleanupFunc 清理函数.
type CleanupFunc func(ctx context.Context) error

// Cleanup 清理任务.
type Cleanup struct {
	Name     string
	Fn       CleanupFunc
	Priority int // 优先级，数字越小越先执行
}

// options 内部配置.
type options struct {
	name            string
	version         string
	logger          logger.Logger
	hooks           *Hooks
	gracefulTimeout time.Duration
	signals         []os.Signal
	cleanups        []Cleanup
}

func defaultOptions() *options {
	return &options{
		name:            "app",
		version:         "1.0.0",
		hooks:           &Hooks{},
		gracefulTimeout: 30 * time.Second,
	}
}

// Option 配置选项.
type Option func(*options)

// WithName 设置应用名称.
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithVersion 设置应用版本.
func WithVersion(version string) Option {
	return func(o *options) { o.version = version }
}

// WithLogger 设置日志记录器（必需）.
func WithLogger(log logger.Logger) Option {
	return func(o *options) { o.logger = log }
}

// WithHooks 设置生命周期钩子.
func WithHooks(hooks *Hooks) Option {
	return func(o *options) { o.hooks = hooks }
}

// WithGracefulTimeout 设置优雅关闭超时时间.
func WithGracefulTimeout(d time.Duration) Option {
	return func(o *options) { o.gracefulTimeout = d }
}

// WithSignals 设置监听的系统信号.
func WithSignals(signals ...os.Signal) Option {
	return func(o *options) { o.signals = signals }
}

// WithCleanup 注册清理任务.
func WithCleanup(name string, fn CleanupFunc, priority int) Option {
	return func(o *options) {
		o.cleanups = append(o.cleanups, Cleanup{
			Name:     name,
			Fn:       fn,
			Priority: priority,
		})
	}
}

// WithCloser 注册 io.Closer 作为清理任务.
func WithCloser(name string, closer interface{ Close() error }, priority int) Option {
	return WithCleanup(name, func(_ context.Context) error {
		return closer.Close()
	}, priority)
}
