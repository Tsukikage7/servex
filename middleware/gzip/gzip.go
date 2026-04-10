// Package gzip 提供 HTTP 响应 gzip 压缩中间件.
package gzip

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Options 压缩配置.
type Options struct {
	// Level 压缩级别，取值范围 gzip.NoCompression(-1) 到 gzip.BestCompression(9).
	Level int
	// MinLength 触发压缩的最小响应体字节数.
	MinLength int
	// ExcludePaths 排除的路径前缀列表.
	ExcludePaths []string
	// ExcludeContentTypes 排除的 Content-Type 列表.
	ExcludeContentTypes []string
}

// Option 配置函数.
type Option func(*Options)

// WithLevel 设置压缩级别.
func WithLevel(level int) Option {
	return func(o *Options) { o.Level = level }
}

// WithMinLength 设置触发压缩的最小字节数.
func WithMinLength(n int) Option {
	return func(o *Options) { o.MinLength = n }
}

// WithExcludePaths 设置排除的路径前缀.
func WithExcludePaths(paths ...string) Option {
	return func(o *Options) { o.ExcludePaths = paths }
}

// WithExcludeContentTypes 设置排除的 Content-Type.
func WithExcludeContentTypes(types ...string) Option {
	return func(o *Options) { o.ExcludeContentTypes = types }
}

// defaultOptions 默认配置.
func defaultOptions() Options {
	return Options{
		Level:     gzip.DefaultCompression,
		MinLength: 256,
	}
}

// New 创建 gzip 压缩中间件.
func New(opts ...Option) func(http.Handler) http.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	pool := &sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, o.Level)
			return w
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查客户端是否支持 gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// 检查路径是否被排除
			for _, p := range o.ExcludePaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			gz := pool.Get().(*gzip.Writer)
			defer func() {
				// 归还 pool 前 Reset，避免引用已释放的 ResponseWriter
				gz.Reset(io.Discard)
				pool.Put(gz)
			}()

			gz.Reset(w)

			gw := &gzipResponseWriter{
				ResponseWriter:      w,
				writer:              gz,
				minLength:           o.MinLength,
				excludeContentTypes: o.ExcludeContentTypes,
			}

			defer gw.finish()

			next.ServeHTTP(gw, r)
		})
	}
}

// Handler 便捷包装，直接返回 http.Handler.
func Handler(next http.Handler, opts ...Option) http.Handler {
	return New(opts...)(next)
}

// gzipResponseWriter 包装 http.ResponseWriter 以支持 gzip 压缩.
type gzipResponseWriter struct {
	http.ResponseWriter
	writer              *gzip.Writer
	minLength           int
	excludeContentTypes []string

	buf           []byte // 缓冲区，用于判断是否达到 MinLength
	headerWritten bool
	gzipEnabled   bool
	statusCode    int
	sniffDone     bool // Content-Type 嗅探是否完成
}

// WriteHeader 写入状态码.
func (w *gzipResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	// 延迟写入 header，等 Write 时决定是否启用 gzip
}

// Write 写入响应体.
func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.sniffDone {
		w.buf = append(w.buf, data...)

		// 尚未累积足够数据进行判断
		if len(w.buf) < w.minLength {
			return len(data), nil
		}

		w.sniffDone = true
		w.decideGzip()
		return len(data), w.flushBuf()
	}

	if w.gzipEnabled {
		return w.writer.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

// decideGzip 根据缓冲内容决定是否启用 gzip.
func (w *gzipResponseWriter) decideGzip() {
	ct := w.Header().Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(w.buf)
		w.Header().Set("Content-Type", ct)
	}

	// 检查 Content-Type 是否被排除
	for _, exc := range w.excludeContentTypes {
		if strings.Contains(ct, exc) {
			w.gzipEnabled = false
			w.writeHeader()
			return
		}
	}

	w.gzipEnabled = true
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Del("Content-Length")
	w.writeHeader()
}

// writeHeader 向底层 ResponseWriter 写入状态码.
func (w *gzipResponseWriter) writeHeader() {
	if w.headerWritten {
		return
	}
	w.headerWritten = true
	code := w.statusCode
	if code == 0 {
		code = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(code)
}

// flushBuf 将缓冲区内容写入目标 writer.
func (w *gzipResponseWriter) flushBuf() error {
	if len(w.buf) == 0 {
		return nil
	}
	var err error
	if w.gzipEnabled {
		_, err = w.writer.Write(w.buf)
	} else {
		_, err = w.ResponseWriter.Write(w.buf)
	}
	w.buf = nil
	return err
}

// finish 完成响应写入.
func (w *gzipResponseWriter) finish() {
	if !w.sniffDone {
		// 响应体小于 MinLength，不压缩直接输出
		w.sniffDone = true
		if len(w.buf) > 0 {
			w.writeHeader()
			w.ResponseWriter.Write(w.buf) //nolint:errcheck
		} else {
			w.writeHeader()
		}
		return
	}
	if w.gzipEnabled {
		w.writer.Close() //nolint:errcheck
	}
}

// Flush 支持 http.Flusher 接口.
func (w *gzipResponseWriter) Flush() {
	if w.gzipEnabled {
		w.writer.Flush() //nolint:errcheck
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
