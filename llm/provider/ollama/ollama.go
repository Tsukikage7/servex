// Package ollama 提供 Ollama 本地模型适配器.
// 底层复用 OpenAI 格式接口，Ollama 默认端口 11434.
package ollama

import (
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/provider/openai"
)

const defaultBaseURL = "http://localhost:11434/v1"

// Option Ollama 客户端选项.
type Option func(*options)

type options struct {
	baseURL        string
	model          string
	embeddingModel string
	httpClient     *http.Client
}

// WithBaseURL 设置 Ollama 服务地址（默认 http://localhost:11434/v1）.
func WithBaseURL(url string) Option { return func(o *options) { o.baseURL = url } }

// WithModel 设置默认聊天模型（默认 llama3.2）.
func WithModel(model string) Option { return func(o *options) { o.model = model } }

// WithEmbeddingModel 设置默认嵌入模型.
func WithEmbeddingModel(m string) Option { return func(o *options) { o.embeddingModel = m } }

// WithHTTPClient 设置自定义 HTTP 客户端（用于配置代理、超时等）.
func WithHTTPClient(hc *http.Client) Option { return func(o *options) { o.httpClient = hc } }

// Client Ollama 客户端，底层复用 OpenAI 适配器.
type Client = openai.Client

// New 创建 Ollama 客户端.
// apiKey 对 Ollama 通常为空字符串（本地服务不需要鉴权）.
func New(apiKey string, opts ...Option) *Client {
	o := &options{
		baseURL:    defaultBaseURL,
		model:      "llama3.2",
		httpClient: &http.Client{Timeout: 120 * time.Second}, // 本地模型推理较慢
	}
	for _, opt := range opts {
		opt(o)
	}

	var openOpts []openai.Option
	openOpts = append(openOpts, openai.WithBaseURL(o.baseURL))
	openOpts = append(openOpts, openai.WithModel(o.model))
	if o.embeddingModel != "" {
		openOpts = append(openOpts, openai.WithEmbeddingModel(o.embeddingModel))
	}
	if o.httpClient != nil {
		openOpts = append(openOpts, openai.WithHTTPClient(o.httpClient))
	}
	return openai.New(apiKey, openOpts...)
}

// 编译期接口断言.
var (
	_ llm.ChatModel      = (*Client)(nil)
	_ llm.EmbeddingModel = (*Client)(nil)
)
