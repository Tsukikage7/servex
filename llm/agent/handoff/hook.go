package handoff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ErrNilHookFunc 使用 NewFuncHook 时传入了 nil 函数.
var ErrNilHookFunc = errors.New("handoff: nil hook func")

// ──────────────────────────────────────────
// FuncHook
// ──────────────────────────────────────────

// funcHook 把任意函数包装为 Hook.
type funcHook struct {
	fn func(ctx context.Context, sig *Signal, input DetectInput) error
}

// NewFuncHook 把任意函数包装为 Hook.
//
// fn 为 nil 时返回 (nil, ErrNilHookFunc). 该 Hook 在高频路径上仅做函数转发，
// 适合内联业务逻辑（如写工单表、发消息队列）.
func NewFuncHook(fn func(ctx context.Context, sig *Signal, input DetectInput) error) (Hook, error) {
	if fn == nil {
		return nil, ErrNilHookFunc
	}
	return &funcHook{fn: fn}, nil
}

// Fire 转调底层函数.
func (h *funcHook) Fire(ctx context.Context, sig *Signal, input DetectInput) error {
	return h.fn(ctx, sig, input)
}

// ──────────────────────────────────────────
// WebhookHook
// ──────────────────────────────────────────

// defaultWebhookTimeout webhook POST 的默认超时.
const defaultWebhookTimeout = 5 * time.Second

// HookOption WebhookHook 配置选项.
type HookOption func(*webhookOptions)

// webhookOptions WebhookHook 内部配置.
type webhookOptions struct {
	// client HTTP 客户端；为 nil 时使用带超时的默认客户端.
	client *http.Client
	// timeout 未显式提供 client 时使用该超时构造默认客户端.
	timeout time.Duration
	// headers 额外的 HTTP 头，例如 Authorization.
	headers map[string]string
}

// WithHTTPClient 自定义 HTTP 客户端.
// 传入自定义 client 后 WithTimeout 选项不再生效.
func WithHTTPClient(c *http.Client) HookOption {
	return func(o *webhookOptions) { o.client = c }
}

// WithTimeout 自定义请求超时（默认 5s）.
// 若同时用 WithHTTPClient 传入 client，此选项被忽略.
func WithTimeout(d time.Duration) HookOption {
	return func(o *webhookOptions) { o.timeout = d }
}

// WithHeader 追加一个 HTTP 头（可重复调用）.
func WithHeader(key, value string) HookOption {
	return func(o *webhookOptions) {
		if o.headers == nil {
			o.headers = map[string]string{}
		}
		o.headers[key] = value
	}
}

// webhookHook 把检测命中信号 POST 到指定 URL.
type webhookHook struct {
	url    string
	client *http.Client
	header map[string]string
}

// webhookPayload POST body 的 JSON 结构.
type webhookPayload struct {
	Should     bool              `json:"should"`
	Reason     string            `json:"reason"`
	Meta       map[string]string `json:"meta,omitempty"`
	Question   string            `json:"question,omitempty"`
	Answer     string            `json:"answer,omitempty"`
	History    []llm.Message     `json:"history,omitempty"`
	RetryCount int               `json:"retry_count"`
	LastScore  float32           `json:"last_score"`
}

// NewWebhookHook 创建 Webhook Hook：命中时 POST JSON 到指定 URL.
//
// 默认行为：
//   - 5s 超时（可用 WithTimeout 覆盖；若用 WithHTTPClient 则完全交给 client）
//   - POST Content-Type: application/json
//   - HTTP 2xx 视为成功，其他状态码（含 4xx/5xx）返回 error
//   - 网络/超时错误以 "handoff: webhook: ..." 前缀包裹原错误
func NewWebhookHook(url string, opts ...HookOption) Hook {
	o := &webhookOptions{timeout: defaultWebhookTimeout}
	for _, opt := range opts {
		opt(o)
	}
	client := o.client
	if client == nil {
		client = &http.Client{Timeout: o.timeout}
	}
	return &webhookHook{url: url, client: client, header: o.headers}
}

// Fire POST JSON 到配置的 URL.
func (h *webhookHook) Fire(ctx context.Context, sig *Signal, input DetectInput) error {
	if sig == nil {
		return errors.New("handoff: webhook: nil signal")
	}
	payload := webhookPayload{
		Should:     sig.Should,
		Reason:     sig.Reason,
		Meta:       sig.Meta,
		Question:   input.Question,
		Answer:     input.Answer,
		History:    input.History,
		RetryCount: input.RetryCount,
		LastScore:  input.LastScore,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("handoff: webhook: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("handoff: webhook: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range h.header {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("handoff: webhook: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("handoff: webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}
