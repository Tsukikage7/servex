package retry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// HTTPClient 可重试的 HTTP 客户端.
type HTTPClient struct {
	client    *http.Client
	cfg       *Config
	retryable HTTPRetryableFunc
}

// HTTPRetryableFunc 判断 HTTP 响应是否应该重试.
type HTTPRetryableFunc func(resp *http.Response, err error) bool

// NewHTTPClient 创建可重试的 HTTP 客户端.
// 使用示例:
//
//	client := retry.NewHTTPClient(http.DefaultClient, retry.DefaultConfig())
//	resp, err := client.Do(req)
func NewHTTPClient(client *http.Client, cfg *Config) *HTTPClient {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{
		client:    client,
		cfg:       cfg,
		retryable: DefaultHTTPRetryable,
	}
}

// WithRetryable 设置 HTTP 重试判断函数.
func (c *HTTPClient) WithRetryable(fn HTTPRetryableFunc) *HTTPClient {
	c.retryable = fn
	return c
}

// Do 执行 HTTP 请求，支持重试.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req)
}

// DoWithContext 执行 HTTP 请求，支持重试和上下文控制.
func (c *HTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	// 保存 body 用于重试
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}

	for attempt := 0; attempt < c.cfg.MaxAttempts; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 恢复 body
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 执行请求
		resp, err = c.client.Do(req.WithContext(ctx))

		// 判断是否应该重试
		if !c.retryable(resp, err) {
			return resp, err
		}

		// 关闭响应体以便重用连接
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		// 如果不是最后一次尝试，则等待
		if attempt < c.cfg.MaxAttempts-1 {
			wait := c.cfg.Backoff(attempt, c.cfg.Delay)
			retryTimer := time.NewTimer(wait)
			select {
			case <-retryTimer.C:
				continue
			case <-ctx.Done():
				retryTimer.Stop()
				return nil, ctx.Err()
			}
		}
	}

	return resp, err
}

// DefaultHTTPRetryable 默认的 HTTP 重试判断.
// 重试网络错误和 5xx 响应.
func DefaultHTTPRetryable(resp *http.Response, err error) bool {
	// 网络错误总是重试
	if err != nil {
		return true
	}

	// 5xx 错误重试
	if resp != nil && resp.StatusCode >= 500 {
		return true
	}

	// 429 Too Many Requests 重试
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	return false
}

// RetryOn5xx 仅在 5xx 错误时重试.
func RetryOn5xx(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp != nil && resp.StatusCode >= 500
}

// RetryOnConnectionError 仅在连接错误时重试.
func RetryOnConnectionError(_ *http.Response, err error) bool {
	return err != nil
}

// HTTPMiddleware 返回 HTTP 重试中间件.
// 适用于服务端，在处理失败时自动重试.
// 注意：此中间件通常用于 HTTP 客户端场景较少。
// 服务端重试请求可能导致意外行为，请谨慎使用。
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 服务端重试通常不推荐，因为请求已经到达
			// 这里只是提供一个框架，实际场景可能需要自定义逻辑
			next.ServeHTTP(w, r)
		})
	}
}
