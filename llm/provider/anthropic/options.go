package anthropic

import "net/http"

const defaultBaseURL = "https://api.anthropic.com"
const defaultAnthropicVersion = "2023-06-01"

// Option Anthropic 客户端选项.
type Option func(*Client)

// WithBaseURL 设置 API 基础 URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithModel 设置默认模型名称.
func WithModel(model string) Option {
	return func(c *Client) { c.model = model }
}

// WithHTTPClient 设置自定义 HTTP 客户端.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithAnthropicVersion 设置 Anthropic API 版本头.
func WithAnthropicVersion(version string) Option {
	return func(c *Client) { c.version = version }
}

// WithDefaultMaxTokens 设置默认最大 token 数Anthropic API 必须提供此参数.
func WithDefaultMaxTokens(n int) Option {
	return func(c *Client) { c.defaultMaxTokens = n }
}
