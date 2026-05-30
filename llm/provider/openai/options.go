package openai

import "net/http"

const defaultBaseURL = "https://api.openai.com/v1"

// Option OpenAI 客户端选项.
type Option func(*Client)

// WithBaseURL 设置 API 基础 URL.
// 用于连接兼容 OpenAI 格式的第三方 Provider，如 DeepSeek、Azure 等.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithModel 设置默认模型名称.
func WithModel(model string) Option {
	return func(c *Client) { c.model = model }
}

// WithHTTPClient 设置自定义 HTTP 客户端用于配置代理、超时等.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithOrganization 设置 OpenAI 组织 ID.
func WithOrganization(orgID string) Option {
	return func(c *Client) { c.orgID = orgID }
}

// WithEmbeddingModel 设置默认嵌入模型名称.
func WithEmbeddingModel(model string) Option {
	return func(c *Client) { c.embeddingModel = model }
}
