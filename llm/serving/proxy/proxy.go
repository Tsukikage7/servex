// Package proxy 提供 OpenAI 兼容的 AI API 代理网关.
// 支持多 Provider 注册、按模型名称路由、API Key 鉴权、内容审核、计费等功能.
package proxy

import (
	"errors"
	"net/http"
	"sync"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/safety/moderation"
	"github.com/Tsukikage7/servex/v2/llm/serving/apikey"
	"github.com/Tsukikage7/servex/v2/llm/serving/billing"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 预定义错误.
var (
	// ErrModelNotFound 指定模型未注册.
	ErrModelNotFound = errors.New("proxy: model not found")
	// ErrNoProviders 没有可用的 Provider.
	ErrNoProviders = errors.New("proxy: no providers available")
	// ErrAllProvidersFailed 所有 Provider 调用失败.
	ErrAllProvidersFailed = errors.New("proxy: all providers failed")
)

// ProviderConfig Provider 配置.
type ProviderConfig struct {
	Name     string   `json:"name" yaml:"name"`
	Models   []string `json:"models" yaml:"models"`
	Weight   int      `json:"weight" yaml:"weight"`     // 负载均衡权重
	Priority int      `json:"priority" yaml:"priority"` // 故障转移优先级（越小越高）
}

// providerEntry 内部 Provider 条目.
type providerEntry struct {
	name     string
	model    llm.ChatModel
	models   []string // 该 Provider 支持的模型名列表
	weight   int      // 负载均衡权重
	priority int      // 故障转移优先级
}

// ProviderOption Provider 注册选项.
type ProviderOption func(*providerEntry)

// WithWeight 设置 Provider 的负载均衡权重.
func WithWeight(w int) ProviderOption {
	return func(e *providerEntry) { e.weight = w }
}

// WithPriority 设置 Provider 的故障转移优先级（越小越高）.
func WithPriority(p int) ProviderOption {
	return func(e *providerEntry) { e.priority = p }
}

// Option Proxy 构造选项.
type Option func(*Proxy)

// WithAPIKeyManager 设置 API Key 管理器，用于请求鉴权.
func WithAPIKeyManager(mgr apikey.Manager) Option {
	return func(p *Proxy) { p.keyMgr = mgr }
}

// WithBilling 设置计费引擎，用于记录 token 用量.
func WithBilling(b billing.Billing) Option {
	return func(p *Proxy) { p.billing = b }
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(p *Proxy) { p.log = logger.WithComponent(log, "LLM") }
}

// WithModeration 设置内容审核器，用于过滤有害内容.
func WithModeration(mod moderation.Moderator) Option {
	return func(p *Proxy) { p.moderator = mod }
}

// Proxy AI API 代理.
type Proxy struct {
	mu        sync.RWMutex
	providers []*providerEntry          // 已注册的 Provider 列表
	modelMap  map[string]*providerEntry // 模型名 -> Provider 索引

	keyMgr    apikey.Manager
	billing   billing.Billing
	log       logger.Logger
	moderator moderation.Moderator
}

// New 创建 Proxy 实例，并将初始 providers map 中的每个 ChatModel 注册进去.
// providers 的 key 为 Provider 名称，value 为 ChatModel 实现.
// 初始注册的 Provider 不绑定任何模型名（需通过 RegisterProvider 补充）.
func New(providers map[string]llm.ChatModel, opts ...Option) *Proxy {
	p := &Proxy{
		modelMap: make(map[string]*providerEntry),
	}
	for _, opt := range opts {
		opt(p)
	}
	// 将传入的 providers map 全部注册，初始不绑定模型名
	for name, model := range providers {
		entry := &providerEntry{
			name:   name,
			model:  model,
			weight: 1,
		}
		p.providers = append(p.providers, entry)
	}
	return p
}

// RegisterProvider 注册 Provider 并绑定其支持的模型名列表.
// 同一 Provider 名称若已存在则更新其条目；模型名冲突时后注册者覆盖先注册者.
func (p *Proxy) RegisterProvider(name string, model llm.ChatModel, models []string, opts ...ProviderOption) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry := &providerEntry{
		name:   name,
		model:  model,
		models: models,
		weight: 1,
	}
	for _, opt := range opts {
		opt(entry)
	}

	// 检查是否已有同名 Provider，有则更新
	for i, e := range p.providers {
		if e.name == name {
			p.providers[i] = entry
			// 清理旧模型映射
			for _, m := range e.models {
				if p.modelMap[m] == e {
					delete(p.modelMap, m)
				}
			}
			// 注册新模型映射
			for _, m := range models {
				p.modelMap[m] = entry
			}
			return
		}
	}

	p.providers = append(p.providers, entry)
	for _, m := range models {
		p.modelMap[m] = entry
	}
}

// Route 根据模型名称选择对应的 Provider.
// 若未找到对应 Provider，返回 ErrModelNotFound.
func (p *Proxy) Route(model string) (llm.ChatModel, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.providers) == 0 {
		return nil, ErrNoProviders
	}

	entry, ok := p.modelMap[model]
	if !ok {
		return nil, ErrModelNotFound
	}
	return entry.model, nil
}

// models 返回所有已注册的模型名列表（内部调用，需在持有读锁时使用）.
func (p *Proxy) listModels() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, 0, len(p.modelMap))
	for name := range p.modelMap {
		names = append(names, name)
	}
	return names
}

// Handler 返回 OpenAI 兼容的 HTTP handler.
// 注册以下路由：
//
//	POST /v1/chat/completions  - 聊天补全
//	GET  /v1/models            - 模型列表
func (p *Proxy) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", p.handleChatCompletion)
	mux.HandleFunc("GET /v1/models", p.handleListModels)
	return mux
}
