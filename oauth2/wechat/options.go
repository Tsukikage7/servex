package wechat

import "net/http"

type options struct {
	appID       string
	appSecret   string
	redirectURL string
	httpClient  *http.Client
}

// Option 配置微信 OAuth2 Provider 的选项函数.
type Option func(*options)

// WithAppID 设置微信应用 ID.
func WithAppID(id string) Option { return func(o *options) { o.appID = id } }

// WithAppSecret 设置微信应用密钥.
func WithAppSecret(s string) Option { return func(o *options) { o.appSecret = s } }

// WithRedirectURL 设置 OAuth2 回调地址.
func WithRedirectURL(url string) Option { return func(o *options) { o.redirectURL = url } }

// WithHTTPClient 设置自定义 HTTP 客户端.
func WithHTTPClient(c *http.Client) Option { return func(o *options) { o.httpClient = c } }
