// Package recovery 提供 panic 恢复中间件.
// 支持 HTTP、gRPC 和 Endpoint 三种类型的 panic 恢复.
package recovery

import (
	"fmt"
	"runtime"

	"github.com/Tsukikage7/servex/observability/logger"
)

// Handler 是 panic 处理函数.
// 参数:
//   - ctx: 可选的上下文信息（HTTP 为 *http.Request，gRPC 为 context.Context）
//   - p: panic 值
//   - stack: 堆栈信息
//
// 返回值:
//   - error: 处理后的错误，将返回给调用方
type Handler func(ctx any, p any, stack []byte) error

// Options 配置选项.
type Options struct {
	// Logger 日志记录器，必需.
	Logger logger.Logger

	// Handler 自定义 panic 处理函数.
	// 如果为 nil，使用默认处理（记录日志并返回内部错误）.
	Handler Handler

	// StackSize 堆栈大小，默认 64KB.
	StackSize int

	// StackAll 是否捕获所有 goroutine 的堆栈，默认 false.
	StackAll bool
}

// Option 是配置函数.
type Option func(*Options)

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// WithHandler 设置自定义 panic 处理函数.
func WithHandler(h Handler) Option {
	return func(o *Options) {
		o.Handler = h
	}
}

// WithStackSize 设置堆栈大小.
func WithStackSize(size int) Option {
	return func(o *Options) {
		o.StackSize = size
	}
}

// WithStackAll 设置是否捕获所有 goroutine 的堆栈.
func WithStackAll(all bool) Option {
	return func(o *Options) {
		o.StackAll = all
	}
}

// defaultOptions 返回默认配置.
func defaultOptions() *Options {
	return &Options{
		StackSize: 64 * 1024, // 64KB
		StackAll:  false,
	}
}

// applyOptions 应用配置选项.
func applyOptions(opts []Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// captureStack 捕获堆栈信息.
func captureStack(size int, all bool) []byte {
	stack := make([]byte, size)
	n := runtime.Stack(stack, all)
	return stack[:n]
}

// PanicError 表示 panic 错误.
type PanicError struct {
	// Value 是 panic 的值.
	Value any
	// Stack 是堆栈信息.
	Stack []byte
}

// Error 实现 error 接口.
func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", e.Value)
}

// Unwrap 返回原始错误（如果 panic 值是 error）.
func (e *PanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}
