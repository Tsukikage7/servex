// Package openai 提供 OpenAI API 适配器.
// 兼容 OpenAI 格式的 Provider：DeepSeek、通义千问Qwen、Azure OpenAI 等.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Client OpenAI API 客户端.
// 兼容所有遵循 OpenAI 接口格式的 Provider：DeepSeek、通义千问、Azure OpenAI 等.
type Client struct {
	apiKey         string
	baseURL        string
	model          string
	embeddingModel string
	orgID          string
	httpClient     *http.Client
}

// 编译期接口断言.
var (
	_ llm.ChatModel      = (*Client)(nil)
	_ llm.EmbeddingModel = (*Client)(nil)
)

// New 创建 OpenAI 客户端.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		model:      "gpt-4o",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ─── 请求/响应结构 ──────────────────────────────────────────────────────────

// chatRequest OpenAI 聊天请求.
type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []chatMessage  `json:"messages"`
	Temperature   *float64       `json:"temperature,omitempty"`
	MaxTokens     *int           `json:"max_tokens,omitempty"`
	TopP          *float64       `json:"top_p,omitempty"`
	Stop          []string       `json:"stop,omitzero"`
	Tools         []chatTool     `json:"tools,omitzero"`
	ToolChoice    any            `json:"tool_choice,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

// streamOptions 流式选项.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// chatMessage OpenAI 消息格式.
type chatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"` // string 或 []contentPart
	Name       string         `json:"name,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitzero"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// contentPart OpenAI 多模态内容片段.
type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

// imageURL 图片 URL.
type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// chatTool 工具定义.
type chatTool struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

// functionDef 函数定义.
type functionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// chatToolCall 工具调用.
type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// chatResponse OpenAI 聊天响应.
type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
		Index        int         `json:"index"`
	} `json:"choices"`
	Usage *usageResponse `json:"usage"`
}

// usageResponse 用量统计.
type usageResponse struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
}

// embedRequest 嵌入请求.
type embedRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

// embedResponse 嵌入响应.
type embedResponse struct {
	Object string `json:"object"`
	Model  string `json:"model"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage usageResponse `json:"usage"`
}

// errorResponse OpenAI 错误响应.
type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"` // 可能是 string 或 int
	} `json:"error"`
}

// ─── ChatModel 实现 ─────────────────────────────────────────────────────────

// Generate 非流式生成.
func (c *Client) Generate(ctx context.Context, messages []Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	o := llm.ApplyOptions(opts)
	req, err := c.buildChatRequest(messages, o, false)
	if err != nil {
		return nil, err
	}

	body, statusCode, retryAfter, err := c.do(ctx, "/chat/completions", req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, c.parseError(statusCode, retryAfter, body)
	}

	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai: 解析响应失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai: 响应中没有 choices")
	}

	choice := resp.Choices[0]
	msg := c.convertResponseMessage(choice.Message)

	result := &llm.ChatResponse{
		Message:      msg,
		FinishReason: choice.FinishReason,
		ModelID:      resp.Model,
	}
	if resp.Usage != nil {
		result.Usage = llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		if resp.Usage.PromptTokensDetails != nil {
			result.Usage.CachedTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	// 流式回调在非流模式下的后处理
	if o.StreamFunc != nil {
		_ = o.StreamFunc(ctx, llm.StreamChunk{Delta: msg.Content, FinishReason: choice.FinishReason})
	}

	return result, nil
}

// Stream 流式生成.
func (c *Client) Stream(ctx context.Context, messages []Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	o := llm.ApplyOptions(opts)
	req, err := c.buildChatRequest(messages, o, true)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.buildHTTPRequest(ctx, "/chat/completions", req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: HTTP 请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		retryAfter := llm.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, c.parseError(resp.StatusCode, retryAfter, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELineSize)

	reader := &streamReader{
		scanner: scanner,
		body:    resp.Body,
	}
	return reader, nil
}

// ─── EmbeddingModel 实现 ────────────────────────────────────────────────────

// EmbedTexts 将文本列表转换为向量.
func (c *Client) EmbedTexts(ctx context.Context, texts []string, opts ...llm.CallOption) (*llm.EmbedResponse, error) {
	o := llm.ApplyOptions(opts)

	model := c.embeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}
	if o.Model != "" {
		model = o.Model
	}

	req := embedRequest{
		Model:          model,
		Input:          texts,
		EncodingFormat: "float",
	}

	body, statusCode, retryAfter, err := c.do(ctx, "/embeddings", req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, c.parseError(statusCode, retryAfter, body)
	}

	var resp embedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai: 解析嵌入响应失败: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for _, d := range resp.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	return &llm.EmbedResponse{
		Embeddings: embeddings,
		Usage: llm.Usage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
		ModelID: resp.Model,
	}, nil
}

// ─── 内部辅助方法 ────────────────────────────────────────────────────────────

// Message 是 llm.Message 的类型别名，便于包内使用.
type Message = llm.Message

// buildChatRequest 构建 OpenAI 聊天请求体.
func (c *Client) buildChatRequest(messages []llm.Message, o llm.CallOptions, stream bool) (chatRequest, error) {
	model := c.model
	if o.Model != "" {
		model = o.Model
	}

	req := chatRequest{
		Model:       model,
		Temperature: o.Temperature,
		MaxTokens:   o.MaxTokens,
		TopP:        o.TopP,
		Stop:        o.Stop,
		Stream:      stream,
	}

	if stream {
		req.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	// 转换消息
	req.Messages = make([]chatMessage, 0, len(messages))
	for _, msg := range messages {
		cm, err := c.convertMessage(msg)
		if err != nil {
			return req, err
		}
		req.Messages = append(req.Messages, cm)
	}

	// 转换工具
	if len(o.Tools) > 0 {
		req.Tools = make([]chatTool, 0, len(o.Tools))
		for _, t := range o.Tools {
			req.Tools = append(req.Tools, chatTool{
				Type: "function",
				Function: functionDef{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			})
		}
	}

	// 转换工具选择策略
	if o.ToolChoice != nil {
		switch o.ToolChoice.Type {
		case "auto", "none", "required":
			req.ToolChoice = o.ToolChoice.Type
		case "function":
			if o.ToolChoice.Function != nil {
				req.ToolChoice = map[string]any{
					"type": "function",
					"function": map[string]string{
						"name": o.ToolChoice.Function.Name,
					},
				}
			}
		}
	}

	return req, nil
}

// convertMessage 将 llm.Message 转换为 OpenAI 格式.
func (c *Client) convertMessage(msg llm.Message) (chatMessage, error) {
	cm := chatMessage{
		Role:       string(msg.Role),
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}

	// 多模态内容
	if len(msg.Parts) > 0 {
		parts := make([]contentPart, 0, len(msg.Parts))
		for _, p := range msg.Parts {
			switch p.Type {
			case llm.ContentTypeText:
				parts = append(parts, contentPart{Type: "text", Text: p.Text})
			case llm.ContentTypeImage:
				parts = append(parts, contentPart{
					Type:     "image_url",
					ImageURL: &imageURL{URL: p.MediaURL},
				})
			}
		}
		cm.Content = parts
	} else {
		cm.Content = msg.Content
	}

	// 工具调用
	if len(msg.ToolCalls) > 0 {
		cm.ToolCalls = make([]chatToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, chatToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return cm, nil
}

// convertResponseMessage 将 OpenAI 响应消息转换为 llm.Message.
func (c *Client) convertResponseMessage(msg chatMessage) llm.Message {
	m := llm.Message{
		Role: llm.Role(msg.Role),
		Name: msg.Name,
	}

	// 内容可能是 string 或 []contentPart
	switch v := msg.Content.(type) {
	case string:
		m.Content = v
	case []any:
		// 多模态内容
		for _, item := range v {
			if part, ok := item.(map[string]any); ok {
				if part["type"] == "text" {
					if text, ok := part["text"].(string); ok {
						m.Content += text
					}
				}
			}
		}
	}

	// 工具调用
	if len(msg.ToolCalls) > 0 {
		m.ToolCalls = make([]llm.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			call := llm.ToolCall{ID: tc.ID}
			call.Function.Name = tc.Function.Name
			call.Function.Arguments = tc.Function.Arguments
			m.ToolCalls = append(m.ToolCalls, call)
		}
	}

	return m
}

// do 执行 HTTP POST 请求，返回响应体字节、状态码和重试延迟.
func (c *Client) do(ctx context.Context, path string, payload any) ([]byte, int, int, error) {
	httpReq, err := c.buildHTTPRequest(ctx, path, payload)
	if err != nil {
		return nil, 0, 0, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("openai: HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 限制响应体读取大小1MB，防止异常响应消耗过多内存.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("openai: 读取响应失败: %w", err)
	}

	retryAfter := llm.ParseRetryAfter(resp.Header.Get("Retry-After"))
	return body, resp.StatusCode, retryAfter, nil
}

// buildHTTPRequest 构建 HTTP 请求.
func (c *Client) buildHTTPRequest(ctx context.Context, path string, payload any) (*http.Request, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openai: 序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("openai: 创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.orgID != "" {
		req.Header.Set("OpenAI-Organization", c.orgID)
	}

	return req, nil
}

// parseError 将 HTTP 错误响应转换为 APIError.
func (c *Client) parseError(statusCode, retryAfter int, body []byte) error {
	var errResp errorResponse
	_ = json.Unmarshal(body, &errResp)

	code := ""
	switch v := errResp.Error.Code.(type) {
	case string:
		code = v
	case float64:
		code = strconv.Itoa(int(v))
	}

	apiErr := &llm.APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    errResp.Error.Message,
		Provider:   "openai",
		RetryAfter: retryAfter,
	}

	switch statusCode {
	case 401, 403:
		return fmt.Errorf("%w: %w", llm.ErrInvalidAuth, apiErr)
	case 429:
		return fmt.Errorf("%w: %w", llm.ErrRateLimited, apiErr)
	case 400:
		if code == "context_length_exceeded" || errResp.Error.Type == "invalid_request_error" {
			if len(body) > 0 {
				return fmt.Errorf("%w: %w", llm.ErrContextLength, apiErr)
			}
		}
		return apiErr
	default:
		if statusCode >= 500 {
			return fmt.Errorf("%w: %w", llm.ErrProviderUnavailable, apiErr)
		}
		return apiErr
	}
}
