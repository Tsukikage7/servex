// Package anthropic 提供 Anthropic Claude API 适配器.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Client Anthropic Claude API 客户端.
type Client struct {
	apiKey           string
	baseURL          string
	model            string
	version          string
	defaultMaxTokens int
	httpClient       *http.Client
}

// 编译期接口断言.
var _ llm.ChatModel = (*Client)(nil)

// New 创建 Anthropic 客户端.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:           apiKey,
		baseURL:          defaultBaseURL,
		model:            "claude-3-5-sonnet-20241022",
		version:          defaultAnthropicVersion,
		defaultMaxTokens: 4096,
		httpClient:       &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ─── 请求/响应结构 ──────────────────────────────────────────────────────────

// messagesRequest Anthropic Messages API 请求.
type messagesRequest struct {
	Model         string          `json:"model"`
	Messages      []anthropicMsg  `json:"messages"`
	System        string          `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitzero"`
	Tools         []anthropicTool `json:"tools,omitzero"`
	ToolChoice    any             `json:"tool_choice,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
}

// anthropicMsg Anthropic 消息格式.
type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string 或 []anthropicContent
}

// anthropicContent Anthropic 内容片段.
type anthropicContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *imageSource    `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

// imageSource 图片来源.
type imageSource struct {
	Type      string `json:"type"` // "base64" 或 "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// anthropicTool Anthropic 工具定义.
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// messagesResponse Anthropic 响应.
type messagesResponse struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Role       string            `json:"role"`
	Model      string            `json:"model"`
	Content    []responseContent `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// responseContent 响应内容.
type responseContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// errorResponse Anthropic 错误响应.
type errorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ─── ChatModel 实现 ─────────────────────────────────────────────────────────

// Generate 非流式生成.
func (c *Client) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	o := llm.ApplyOptions(opts)
	req, system := c.buildRequest(messages, o, false)

	body, statusCode, retryAfter, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, c.parseError(statusCode, retryAfter, body)
	}

	var resp messagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("anthropic: 解析响应失败: %w", err)
	}
	_ = system

	msg := c.convertResponse(resp)
	return &llm.ChatResponse{
		Message:      msg,
		FinishReason: resp.StopReason,
		ModelID:      resp.Model,
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

// Stream 流式生成.
func (c *Client) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	o := llm.ApplyOptions(opts)
	req, _ := c.buildRequest(messages, o, true)

	httpReq, err := c.buildHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP 请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		retryAfter := llm.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, c.parseError(resp.StatusCode, retryAfter, body)
	}

	return &streamReader{
		scanner: bufio.NewScanner(resp.Body),
		body:    resp.Body,
	}, nil
}

// ─── 内部辅助方法 ────────────────────────────────────────────────────────────

// buildRequest 构建 Anthropic 请求体，返回请求和提取出的系统提示.
func (c *Client) buildRequest(messages []llm.Message, o llm.CallOptions, stream bool) (messagesRequest, string) {
	model := c.model
	if o.Model != "" {
		model = o.Model
	}

	maxTokens := c.defaultMaxTokens
	if o.MaxTokens != nil {
		maxTokens = *o.MaxTokens
	}

	req := messagesRequest{
		Model:         model,
		MaxTokens:     maxTokens,
		Temperature:   o.Temperature,
		TopP:          o.TopP,
		StopSequences: o.Stop,
		Stream:        stream,
	}

	// 提取系统提示（Anthropic 要求系统消息单独作为 system 字段）
	var system string
	for _, msg := range messages {
		if msg.Role == llm.RoleSystem {
			system = msg.Content
			req.System = system
			continue
		}
		am, err := c.convertMessage(msg)
		if err == nil {
			req.Messages = append(req.Messages, am)
		}
	}

	// 转换工具
	if len(o.Tools) > 0 {
		req.Tools = make([]anthropicTool, 0, len(o.Tools))
		for _, t := range o.Tools {
			req.Tools = append(req.Tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
	}

	// 工具选择
	if o.ToolChoice != nil {
		switch o.ToolChoice.Type {
		case "auto":
			req.ToolChoice = map[string]string{"type": "auto"}
		case "none":
			req.ToolChoice = map[string]string{"type": "none"}
		case "required":
			req.ToolChoice = map[string]string{"type": "any"}
		case "function":
			if o.ToolChoice.Function != nil {
				req.ToolChoice = map[string]any{
					"type": "tool",
					"name": o.ToolChoice.Function.Name,
				}
			}
		}
	}

	return req, system
}

// convertMessage 将 llm.Message 转换为 Anthropic 格式.
func (c *Client) convertMessage(msg llm.Message) (anthropicMsg, error) {
	am := anthropicMsg{Role: string(msg.Role)}

	// 工具结果消息
	if msg.Role == llm.RoleTool {
		am.Content = []anthropicContent{{
			Type:      "tool_result",
			ToolUseID: msg.ToolCallID,
			Content:   msg.Content,
		}}
		return am, nil
	}

	// 助手消息（可能含工具调用）
	if msg.Role == llm.RoleAssistant {
		if len(msg.ToolCalls) > 0 {
			contents := make([]anthropicContent, 0, 1+len(msg.ToolCalls))
			if msg.Content != "" {
				contents = append(contents, anthropicContent{Type: "text", Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				input := json.RawMessage(tc.Function.Arguments)
				if len(input) == 0 {
					input = json.RawMessage(`{}`)
				}
				contents = append(contents, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			am.Content = contents
			return am, nil
		}
		am.Content = msg.Content
		return am, nil
	}

	// 用户消息（可能含多模态内容）
	if len(msg.Parts) > 0 {
		contents := make([]anthropicContent, 0, len(msg.Parts))
		for _, p := range msg.Parts {
			switch p.Type {
			case llm.ContentTypeText:
				contents = append(contents, anthropicContent{Type: "text", Text: p.Text})
			case llm.ContentTypeImage:
				contents = append(contents, anthropicContent{
					Type: "image",
					Source: &imageSource{
						Type: "url",
						URL:  p.MediaURL,
					},
				})
			}
		}
		am.Content = contents
		return am, nil
	}

	am.Content = msg.Content
	return am, nil
}

// convertResponse 将 Anthropic 响应内容转换为 llm.Message.
func (c *Client) convertResponse(resp messagesResponse) llm.Message {
	msg := llm.Message{Role: llm.RoleAssistant}
	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			msg.Content += content.Text
		case "tool_use":
			call := llm.ToolCall{ID: content.ID}
			call.Function.Name = content.Name
			call.Function.Arguments = string(content.Input)
			msg.ToolCalls = append(msg.ToolCalls, call)
		}
	}
	return msg
}

// do 执行 HTTP 请求，返回响应体、状态码和重试延迟.
func (c *Client) do(ctx context.Context, payload any) ([]byte, int, int, error) {
	httpReq, err := c.buildHTTPRequest(ctx, payload)
	if err != nil {
		return nil, 0, 0, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("anthropic: HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 限制响应体读取大小（1MB），防止异常响应消耗过多内存.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("anthropic: 读取响应失败: %w", err)
	}

	retryAfter := llm.ParseRetryAfter(resp.Header.Get("Retry-After"))
	return body, resp.StatusCode, retryAfter, nil
}

// buildHTTPRequest 构建 HTTP 请求.
func (c *Client) buildHTTPRequest(ctx context.Context, payload any) (*http.Request, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anthropic: 序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("anthropic: 创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.version)

	return req, nil
}

// parseError 将错误响应转换为 APIError.
func (c *Client) parseError(statusCode, retryAfter int, body []byte) error {
	var errResp errorResponse
	_ = json.Unmarshal(body, &errResp)

	apiErr := &llm.APIError{
		StatusCode: statusCode,
		Code:       errResp.Error.Type,
		Message:    errResp.Error.Message,
		Provider:   "anthropic",
		RetryAfter: retryAfter,
	}

	switch statusCode {
	case 401:
		return fmt.Errorf("%w: %w", llm.ErrInvalidAuth, apiErr)
	case 429:
		return fmt.Errorf("%w: %w", llm.ErrRateLimited, apiErr)
	default:
		if statusCode >= 500 {
			return fmt.Errorf("%w: %w", llm.ErrProviderUnavailable, apiErr)
		}
		return apiErr
	}
}
