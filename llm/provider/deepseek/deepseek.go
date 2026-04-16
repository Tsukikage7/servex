// Package deepseek 提供 DeepSeek API 适配器.
// 底层复用 OpenAI 格式接口.
package deepseek

import (
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	openllm "github.com/Tsukikage7/servex/v2/llm/provider/openai"
)

const defaultBaseURL = "https://api.deepseek.com/v1"

// Option DeepSeek 客户端选项.
type Option func(*options)

type options struct {
	baseURL        string
	model          string
	embeddingModel string
	httpClient     *http.Client
}

// WithBaseURL 设置 DeepSeek API 基础 URL（默认 https://api.deepseek.com/v1）.
func WithBaseURL(url string) Option { return func(o *options) { o.baseURL = url } }

// WithModel 设置默认聊天模型（默认 deepseek-chat）.
func WithModel(model string) Option { return func(o *options) { o.model = model } }

// WithEmbeddingModel 设置默认嵌入模型.
func WithEmbeddingModel(m string) Option { return func(o *options) { o.embeddingModel = m } }

// WithHTTPClient 设置自定义 HTTP 客户端（用于配置代理、超时等）.
func WithHTTPClient(hc *http.Client) Option { return func(o *options) { o.httpClient = hc } }

// Client DeepSeek 客户端，底层复用 OpenAI 适配器.
type Client = openllm.Client

// New 创建 DeepSeek 客户端.
// apiKey 为 DeepSeek 平台的 API Key，必填.
func New(apiKey string, opts ...Option) *Client {
	o := &options{
		baseURL:    defaultBaseURL,
		model:      "deepseek-chat",
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(o)
	}

	var openOpts []openllm.Option
	openOpts = append(openOpts, openllm.WithBaseURL(o.baseURL))
	openOpts = append(openOpts, openllm.WithModel(o.model))
	if o.embeddingModel != "" {
		openOpts = append(openOpts, openllm.WithEmbeddingModel(o.embeddingModel))
	}
	if o.httpClient != nil {
		openOpts = append(openOpts, openllm.WithHTTPClient(o.httpClient))
	}
	return openllm.New(apiKey, openOpts...)
}

// 编译期接口断言.
var (
	_ llm.ChatModel      = (*Client)(nil)
	_ llm.EmbeddingModel = (*Client)(nil)
)
