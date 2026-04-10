// Package gemini 提供 Google Gemini REST API 适配器.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Tsukikage7/servex/llm"
)

// Client Google Gemini REST API 客户端.
type Client struct {
	apiKey         string
	baseURL        string
	model          string
	embeddingModel string
	httpClient     *http.Client
}

// 编译期接口断言.
var (
	_ llm.ChatModel      = (*Client)(nil)
	_ llm.EmbeddingModel = (*Client)(nil)
)

// New 创建 Gemini 客户端.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		model:      "gemini-2.0-flash",
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ─── 请求/响应结构 ──────────────────────────────────────────────────────────

// generateRequest Gemini generateContent 请求.
type generateRequest struct {
	Contents          []geminiContent   `json:"contents"`
	SystemInstruction *geminiContent    `json:"systemInstruction,omitempty"`
	Tools             []geminiTool      `json:"tools,omitzero"`
	ToolConfig        *toolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

// geminiContent Gemini 内容.
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart Gemini 内容片段.
type geminiPart struct {
	Text             string        `json:"text,omitempty"`
	InlineData       *inlineData   `json:"inlineData,omitempty"`
	FunctionCall     *functionCall `json:"functionCall,omitempty"`
	FunctionResponse *funcResponse `json:"functionResponse,omitempty"`
}

// inlineData 内联数据（图片等）.
type inlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

// functionCall 函数调用.
type functionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// funcResponse 函数调用结果.
type funcResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// geminiTool Gemini 工具定义.
type geminiTool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations,omitzero"`
}

// functionDeclaration 函数声明.
type functionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// toolConfig 工具配置.
type toolConfig struct {
	FunctionCallingConfig struct {
		Mode                 string   `json:"mode"` // AUTO, ANY, NONE
		AllowedFunctionNames []string `json:"allowedFunctionNames,omitzero"`
	} `json:"functionCallingConfig"`
}

// generationConfig 生成配置.
type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitzero"`
}

// generateResponse Gemini 响应.
type generateResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
		Index        int           `json:"index"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

// embedRequest Gemini 嵌入请求.
type embedRequest struct {
	Model   string        `json:"model"`
	Content geminiContent `json:"content"`
}

// batchEmbedRequest 批量嵌入请求.
type batchEmbedRequest struct {
	Requests []embedRequest `json:"requests"`
}

// batchEmbedResponse 批量嵌入响应.
type batchEmbedResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

// errorResponse Gemini 错误响应.
type errorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// ─── ChatModel 实现 ─────────────────────────────────────────────────────────

// Generate 非流式生成.
func (c *Client) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	o := llm.ApplyOptions(opts)
	model := c.resolveModel(o.Model)
	req := c.buildRequest(messages, o)

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)
	body, statusCode, retryAfter, err := c.do(ctx, url, req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, c.parseError(statusCode, retryAfter, body)
	}

	var resp generateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("gemini: 解析响应失败: %w", err)
	}
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini: 响应中没有 candidates")
	}

	candidate := resp.Candidates[0]
	msg := c.convertResponse(candidate.Content)

	result := &llm.ChatResponse{
		Message:      msg,
		FinishReason: candidate.FinishReason,
		ModelID:      resp.ModelVersion,
	}
	if resp.UsageMetadata != nil {
		result.Usage = llm.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}
	return result, nil
}

// Stream 流式生成.
func (c *Client) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	o := llm.ApplyOptions(opts)
	model := c.resolveModel(o.Model)
	req := c.buildRequest(messages, o)

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, model, c.apiKey)
	httpReq, err := c.buildHTTPRequest(ctx, url, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: HTTP 请求失败: %w", err)
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

// ─── EmbeddingModel 实现 ────────────────────────────────────────────────────

// EmbedTexts 将文本列表转换为向量.
func (c *Client) EmbedTexts(ctx context.Context, texts []string, opts ...llm.CallOption) (*llm.EmbedResponse, error) {
	o := llm.ApplyOptions(opts)
	model := c.embeddingModel
	if model == "" {
		model = "text-embedding-004"
	}
	if o.Model != "" {
		model = o.Model
	}

	requests := make([]embedRequest, 0, len(texts))
	for _, text := range texts {
		requests = append(requests, embedRequest{
			Model: "models/" + model,
			Content: geminiContent{
				Parts: []geminiPart{{Text: text}},
			},
		})
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:batchEmbedContents?key=%s", c.baseURL, model, c.apiKey)
	body, statusCode, retryAfter, err := c.do(ctx, url, batchEmbedRequest{Requests: requests})
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, c.parseError(statusCode, retryAfter, body)
	}

	var resp batchEmbedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("gemini: 解析嵌入响应失败: %w", err)
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		embeddings[i] = e.Values
	}

	return &llm.EmbedResponse{
		Embeddings: embeddings,
		ModelID:    model,
	}, nil
}

// ─── 内部辅助方法 ────────────────────────────────────────────────────────────

// resolveModel 解析最终使用的模型名称.
func (c *Client) resolveModel(optModel string) string {
	if optModel != "" {
		return optModel
	}
	return c.model
}

// buildRequest 构建 Gemini 请求体.
func (c *Client) buildRequest(messages []llm.Message, o llm.CallOptions) generateRequest {
	req := generateRequest{}

	// 提取系统提示
	var userMessages []llm.Message
	for _, msg := range messages {
		if msg.Role == llm.RoleSystem {
			req.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
			continue
		}
		userMessages = append(userMessages, msg)
	}

	// 转换消息
	for _, msg := range userMessages {
		if cm := c.convertMessage(msg); cm != nil {
			req.Contents = append(req.Contents, *cm)
		}
	}

	// 生成配置
	if o.Temperature != nil || o.MaxTokens != nil || o.TopP != nil || len(o.Stop) > 0 {
		req.GenerationConfig = &generationConfig{
			Temperature:     o.Temperature,
			TopP:            o.TopP,
			MaxOutputTokens: o.MaxTokens,
			StopSequences:   o.Stop,
		}
	}

	// 工具
	if len(o.Tools) > 0 {
		decls := make([]functionDeclaration, 0, len(o.Tools))
		for _, t := range o.Tools {
			decls = append(decls, functionDeclaration{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}
		req.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	// 工具选择
	if o.ToolChoice != nil {
		tc := &toolConfig{}
		switch o.ToolChoice.Type {
		case "auto":
			tc.FunctionCallingConfig.Mode = "AUTO"
		case "none":
			tc.FunctionCallingConfig.Mode = "NONE"
		case "required":
			tc.FunctionCallingConfig.Mode = "ANY"
		case "function":
			tc.FunctionCallingConfig.Mode = "ANY"
			if o.ToolChoice.Function != nil {
				tc.FunctionCallingConfig.AllowedFunctionNames = []string{o.ToolChoice.Function.Name}
			}
		}
		req.ToolConfig = tc
	}

	return req
}

// convertMessage 将 llm.Message 转换为 Gemini 格式.
func (c *Client) convertMessage(msg llm.Message) *geminiContent {
	role := "user"
	if msg.Role == llm.RoleAssistant {
		role = "model"
	}

	content := &geminiContent{Role: role}

	// 工具调用结果
	if msg.Role == llm.RoleTool {
		var result map[string]any
		_ = json.Unmarshal([]byte(msg.Content), &result)
		if result == nil {
			result = map[string]any{"output": msg.Content}
		}
		content.Parts = []geminiPart{{
			FunctionResponse: &funcResponse{
				Name:     msg.ToolCallID, // 使用 ToolCallID 作为函数名（Gemini 要求）
				Response: result,
			},
		}}
		return content
	}

	// 助手工具调用
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			content.Parts = append(content.Parts, geminiPart{
				FunctionCall: &functionCall{
					Name: tc.Function.Name,
					Args: args,
				},
			})
		}
		return content
	}

	// 多模态内容
	if len(msg.Parts) > 0 {
		for _, p := range msg.Parts {
			switch p.Type {
			case llm.ContentTypeText:
				content.Parts = append(content.Parts, geminiPart{Text: p.Text})
			case llm.ContentTypeImage:
				content.Parts = append(content.Parts, geminiPart{
					InlineData: &inlineData{
						MIMEType: p.MIMEType,
						Data:     p.MediaURL,
					},
				})
			}
		}
		return content
	}

	content.Parts = []geminiPart{{Text: msg.Content}}
	return content
}

// convertResponse 将 Gemini 响应转换为 llm.Message.
func (c *Client) convertResponse(content geminiContent) llm.Message {
	msg := llm.Message{Role: llm.RoleAssistant}
	for _, part := range content.Parts {
		if part.Text != "" {
			msg.Content += part.Text
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			call := llm.ToolCall{}
			call.Function.Name = part.FunctionCall.Name
			call.Function.Arguments = string(args)
			msg.ToolCalls = append(msg.ToolCalls, call)
		}
	}
	return msg
}

// do 执行 HTTP 请求.
func (c *Client) do(ctx context.Context, url string, payload any) ([]byte, int, int, error) {
	req, err := c.buildHTTPRequest(ctx, url, payload)
	if err != nil {
		return nil, 0, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("gemini: HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 限制响应体读取大小（1MB），防止异常响应消耗过多内存.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("gemini: 读取响应失败: %w", err)
	}

	retryAfter := llm.ParseRetryAfter(resp.Header.Get("Retry-After"))
	return body, resp.StatusCode, retryAfter, nil
}

// buildHTTPRequest 构建 HTTP 请求.
func (c *Client) buildHTTPRequest(ctx context.Context, url string, payload any) (*http.Request, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gemini: 序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gemini: 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// parseError 将错误响应转换为 APIError.
func (c *Client) parseError(statusCode, retryAfter int, body []byte) error {
	var errResp errorResponse
	_ = json.Unmarshal(body, &errResp)

	apiErr := &llm.APIError{
		StatusCode: statusCode,
		Code:       errResp.Error.Status,
		Message:    errResp.Error.Message,
		Provider:   "gemini",
		RetryAfter: retryAfter,
	}

	switch statusCode {
	case 401, 403:
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
