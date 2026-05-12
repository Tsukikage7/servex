// Package logging 提供 HTTP 请求日志中间件.
package logging

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Options 配置选项.
type Options struct {
	// Logger 日志记录器。未设置时会回退到 context 中的 logger 或空日志实现.
	Logger logger.Logger

	// SkipPaths 跳过记录的路径列表（如 /health、/metrics）.
	SkipPaths []string
}

// Option 配置函数.
type Option func(*Options)

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// WithSkipPaths 设置不记录日志的路径.
func WithSkipPaths(paths ...string) Option {
	return func(o *Options) {
		o.SkipPaths = append(o.SkipPaths, paths...)
	}
}

func applyOptions(opts []Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func shouldSkip(path string, skipPaths []string) bool {
	for _, p := range skipPaths {
		if p == path {
			return true
		}
	}
	return false
}

// statusRecorder 捕获响应状态码和写入字节数.
type statusRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += n
	return n, err
}

// Flush 实现 http.Flusher，透传给底层 ResponseWriter.
// SSE 等流式响应依赖此方法将数据块实时推送给客户端.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
