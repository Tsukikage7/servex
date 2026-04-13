package nwebhook

import (
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

type senderOptions struct {
	httpClient *http.Client
	timeout    time.Duration
	maxRetry   int
	logger     logger.Logger
}

// Option 通知 Webhook 发送器配置选项.
type Option func(*senderOptions)

// WithTimeout 设置 HTTP 请求超时时间.
func WithTimeout(d time.Duration) Option { return func(o *senderOptions) { o.timeout = d } }

// WithRetry 设置最大重试次数.
func WithRetry(n int) Option { return func(o *senderOptions) { o.maxRetry = n } }

// WithHTTPClient 设置自定义 HTTP 客户端.
func WithHTTPClient(c *http.Client) Option { return func(o *senderOptions) { o.httpClient = c } }

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option { return func(o *senderOptions) { o.logger = log } }
