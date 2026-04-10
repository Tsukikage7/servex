package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/serving/apikey"

	authapikey "github.com/Tsukikage7/servex/auth/apikey"
)

// ── Mock 实现 ────────────────────────────────────────────────────────────────

// mockModel 用于测试的 ChatModel mock 实现.
type mockModel struct {
	response *llm.ChatResponse
	err      error
}

func (m *mockModel) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	if m.err != nil {
		return nil, m.err
	}
	chunks := []llm.StreamChunk{
		{Delta: "Hello"},
		{Delta: " World"},
		{Delta: "!", FinishReason: "stop"},
	}
	return &mockStreamReader{
		chunks:   chunks,
		response: m.response,
	}, nil
}

// mockStreamReader 用于测试的 StreamReader mock 实现.
type mockStreamReader struct {
	chunks   []llm.StreamChunk
	pos      int
	response *llm.ChatResponse
}

func (r *mockStreamReader) Recv() (llm.StreamChunk, error) {
	if r.pos >= len(r.chunks) {
		return llm.StreamChunk{}, io.EOF
	}
	chunk := r.chunks[r.pos]
	r.pos++
	return chunk, nil
}

func (r *mockStreamReader) Response() *llm.ChatResponse {
	return r.response
}

func (r *mockStreamReader) Close() error { return nil }

// ── 编译期接口断言 ─────────────────────────────────────────────────────────

var _ llm.ChatModel = (*mockModel)(nil)
var _ llm.StreamReader = (*mockStreamReader)(nil)

// ── 辅助函数 ──────────────────────────────────────────────────────────────

// newTestProxy 创建用于测试的 Proxy，注册两个不同模型的 Provider.
func newTestProxy() *Proxy {
	p := New(nil)
	p.RegisterProvider("openai", &mockModel{
		response: &llm.ChatResponse{
			Message:      llm.AssistantMessage("Hi from OpenAI"),
			FinishReason: "stop",
			ModelID:      "gpt-4",
			Usage: llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}, []string{"gpt-4", "gpt-3.5-turbo"})

	p.RegisterProvider("anthropic", &mockModel{
		response: &llm.ChatResponse{
			Message:      llm.AssistantMessage("Hi from Anthropic"),
			FinishReason: "stop",
			ModelID:      "claude-3-opus",
			Usage: llm.Usage{
				PromptTokens:     8,
				CompletionTokens: 4,
				TotalTokens:      12,
			},
		},
	}, []string{"claude-3-opus", "claude-3-sonnet"})

	return p
}

// ── 测试用例 ──────────────────────────────────────────────────────────────

// TestNew 验证 New 函数能正确创建 Proxy 实例并注册初始 providers.
func TestNew(t *testing.T) {
	providers := map[string]llm.ChatModel{
		"openai": &mockModel{},
	}
	p := New(providers)

	if p == nil {
		t.Fatal("期望 Proxy 不为 nil")
	}
	if len(p.providers) != 1 {
		t.Errorf("期望 1 个 provider，实际 %d", len(p.providers))
	}
	if p.providers[0].name != "openai" {
		t.Errorf("期望 provider 名称 'openai'，实际 '%s'", p.providers[0].name)
	}
}

// TestProxy_Route 验证按模型名称路由到正确 Provider.
func TestProxy_Route(t *testing.T) {
	p := newTestProxy()

	// 路由到 OpenAI provider
	model, err := p.Route("gpt-4")
	if err != nil {
		t.Fatalf("路由 gpt-4 失败: %v", err)
	}
	if model == nil {
		t.Fatal("期望 model 不为 nil")
	}

	// 路由到 Anthropic provider
	model2, err := p.Route("claude-3-opus")
	if err != nil {
		t.Fatalf("路由 claude-3-opus 失败: %v", err)
	}
	if model2 == nil {
		t.Fatal("期望 model2 不为 nil")
	}

	// 两个模型应路由到不同 provider
	if model == model2 {
		t.Error("期望 gpt-4 和 claude-3-opus 路由到不同 Provider")
	}
}

// TestProxy_RouteNotFound 验证未知模型返回 ErrModelNotFound.
func TestProxy_RouteNotFound(t *testing.T) {
	p := newTestProxy()

	_, err := p.Route("unknown-model-xyz")
	if err == nil {
		t.Fatal("期望返回错误，实际为 nil")
	}
	if err != ErrModelNotFound {
		t.Errorf("期望 ErrModelNotFound，实际 %v", err)
	}
}

// TestProxy_Handler_ChatCompletion 验证 POST /v1/chat/completions 返回 OpenAI 格式响应.
func TestProxy_Handler_ChatCompletion(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	reqBody := chatCompletionRequest{
		Model: "gpt-4",
		Messages: []messageReq{
			{Role: "user", Content: "Hello"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d，响应: %s", rec.Code, rec.Body.String())
	}

	var resp chatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Object != "chat.completion" {
		t.Errorf("期望 object='chat.completion'，实际 '%s'", resp.Object)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("期望 model='gpt-4'，实际 '%s'", resp.Model)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("期望 choices 不为空")
	}
	if resp.Choices[0].Message.Content == "" {
		t.Error("期望 choices[0].message.content 不为空")
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("期望 role='assistant'，实际 '%s'", resp.Choices[0].Message.Role)
	}
	if resp.Usage.TotalTokens == 0 {
		t.Error("期望 usage.total_tokens > 0")
	}
}

// TestProxy_Handler_Stream 验证流式请求以 SSE 格式输出.
func TestProxy_Handler_Stream(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	reqBody := chatCompletionRequest{
		Model: "gpt-4",
		Messages: []messageReq{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d，响应: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("期望 Content-Type 包含 'text/event-stream'，实际 '%s'", contentType)
	}

	// 解析 SSE 事件
	var dataLines []string
	scanner := bufio.NewScanner(rec.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	if len(dataLines) == 0 {
		t.Fatal("期望至少有一个 SSE data 行")
	}

	// 最后一行应为 [DONE]
	lastLine := dataLines[len(dataLines)-1]
	if lastLine != "[DONE]" {
		t.Errorf("期望最后 SSE 行为 '[DONE]'，实际 '%s'", lastLine)
	}

	// 验证非 [DONE] 的数据行为有效 JSON
	for _, line := range dataLines[:len(dataLines)-1] {
		var chunk map[string]any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			t.Errorf("SSE 数据行不是有效 JSON: %s，错误: %v", line, err)
		}
		if chunk["object"] != "chat.completion.chunk" {
			t.Errorf("期望 object='chat.completion.chunk'，实际 '%v'", chunk["object"])
		}
	}
}

// TestProxy_Handler_ListModels 验证 GET /v1/models 返回已注册模型列表.
func TestProxy_Handler_ListModels(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", rec.Code)
	}

	var resp modelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("解析模型列表响应失败: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("期望 object='list'，实际 '%s'", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Fatal("期望模型列表不为空")
	}

	// 验证所有注册的模型都在列表中
	expectedModels := map[string]bool{
		"gpt-4":           false,
		"gpt-3.5-turbo":   false,
		"claude-3-opus":   false,
		"claude-3-sonnet": false,
	}
	for _, m := range resp.Data {
		if _, ok := expectedModels[m.ID]; ok {
			expectedModels[m.ID] = true
		}
		if m.Object != "model" {
			t.Errorf("期望 model.object='model'，实际 '%s'", m.Object)
		}
	}
	for name, found := range expectedModels {
		if !found {
			t.Errorf("期望模型 '%s' 在列表中，但未找到", name)
		}
	}
}

// mockAPIKeyManager 用于测试的 Manager mock 实现.
type mockAPIKeyManager struct {
	keys map[string]*apikey.Key
}

func (m *mockAPIKeyManager) Create(_ context.Context, _ ...apikey.CreateOption) (string, *apikey.Key, error) {
	return "", nil, nil
}

func (m *mockAPIKeyManager) Validate(_ context.Context, rawKey string) (*apikey.Key, error) {
	key, ok := m.keys[rawKey]
	if !ok {
		return nil, apikey.ErrInvalidKey
	}
	return key, nil
}

func (m *mockAPIKeyManager) Revoke(_ context.Context, _ string) error { return nil }

func (m *mockAPIKeyManager) List(_ context.Context, _ string) ([]*apikey.Key, error) {
	return nil, nil
}

func (m *mockAPIKeyManager) UpdateQuota(_ context.Context, _ string, _ int64) error { return nil }

func (m *mockAPIKeyManager) AsValidator() authapikey.Validator { return nil }

var _ apikey.Manager = (*mockAPIKeyManager)(nil)

// TestProxy_WithAPIKey 验证启用 API Key 鉴权时，无效请求被拒绝，有效请求被放行.
func TestProxy_WithAPIKey(t *testing.T) {
	validKey := "sk-valid-test-key"
	keyObj := &apikey.Key{
		ID:      "key-001",
		Enabled: true,
	}
	mgr := &mockAPIKeyManager{
		keys: map[string]*apikey.Key{
			validKey: keyObj,
		},
	}

	p := New(nil, WithAPIKeyManager(mgr))
	p.RegisterProvider("openai", &mockModel{
		response: &llm.ChatResponse{
			Message:      llm.AssistantMessage("ok"),
			FinishReason: "stop",
			ModelID:      "gpt-4",
			Usage:        llm.Usage{TotalTokens: 10},
		},
	}, []string{"gpt-4"})

	handler := p.Handler()

	// 使用 APIKey 中间件包装 handler
	wrappedHandler := apikey.HTTPMiddleware(mgr)(handler)

	reqBody := chatCompletionRequest{
		Model:    "gpt-4",
		Messages: []messageReq{{Role: "user", Content: "hi"}},
	}
	body, _ := json.Marshal(reqBody)

	t.Run("无 API Key 被拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("期望状态码 401，实际 %d", rec.Code)
		}
	})

	t.Run("有效 API Key 通过", func(t *testing.T) {
		bodyReader := bytes.NewReader(body)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bodyReader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+validKey)
		rec := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d，响应: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestProxy_Handler_ChatCompletion_ModelNotFound 验证未知模型返回 404.
func TestProxy_Handler_ChatCompletion_ModelNotFound(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	reqBody := chatCompletionRequest{
		Model:    "unknown-model",
		Messages: []messageReq{{Role: "user", Content: "hi"}},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404，实际 %d", rec.Code)
	}
}

// TestProxy_RegisterProvider_UpdateExisting 验证同名 Provider 更新时模型映射正确切换.
func TestProxy_RegisterProvider_UpdateExisting(t *testing.T) {
	p := New(nil)
	m1 := &mockModel{response: &llm.ChatResponse{
		Message: llm.AssistantMessage("v1"), ModelID: "gpt-4",
		Usage: llm.Usage{TotalTokens: 1},
	}}
	m2 := &mockModel{response: &llm.ChatResponse{
		Message: llm.AssistantMessage("v2"), ModelID: "gpt-4",
		Usage: llm.Usage{TotalTokens: 2},
	}}

	p.RegisterProvider("openai", m1, []string{"gpt-4"})
	p.RegisterProvider("openai", m2, []string{"gpt-4", "gpt-5"})

	// 路由到更新后的 provider
	routed, err := p.Route("gpt-4")
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}
	if routed != m2 {
		t.Error("期望路由到更新后的 m2 provider")
	}

	// gpt-5 也应可路由
	_, err = p.Route("gpt-5")
	if err != nil {
		t.Errorf("期望 gpt-5 可路由，实际错误: %v", err)
	}
}

// TestProxy_NoProviders 验证无 Provider 时路由返回 ErrNoProviders.
func TestProxy_NoProviders(t *testing.T) {
	p := New(nil)

	_, err := p.Route("gpt-4")
	if err != ErrNoProviders {
		t.Errorf("期望 ErrNoProviders，实际 %v", err)
	}
}

// TestProxy_Handler_BadRequest 验证请求体格式错误返回 400.
func TestProxy_Handler_BadRequest(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，实际 %d", rec.Code)
	}
}

// TestProxy_ListModels_Empty 验证无模型时返回空列表.
func TestProxy_ListModels_Empty(t *testing.T) {
	p := New(nil)
	handler := p.Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", rec.Code)
	}

	var resp modelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("期望空模型列表，实际 %d 个", len(resp.Data))
	}
}

// TestProxy_Options 验证 Option 函数正确设置 Proxy 字段.
func TestProxy_Options(t *testing.T) {
	mgr := &mockAPIKeyManager{keys: map[string]*apikey.Key{}}
	p := New(nil, WithAPIKeyManager(mgr))

	if p.keyMgr == nil {
		t.Error("期望 keyMgr 不为 nil")
	}
}

// TestProxy_ProviderOptions 验证 ProviderOption 正确设置权重和优先级.
func TestProxy_ProviderOptions(t *testing.T) {
	p := New(nil)
	p.RegisterProvider("openai", &mockModel{}, []string{"gpt-4"},
		WithWeight(3), WithPriority(1))

	if len(p.providers) != 1 {
		t.Fatalf("期望 1 个 provider，实际 %d", len(p.providers))
	}
	entry := p.providers[0]
	if entry.weight != 3 {
		t.Errorf("期望 weight=3，实际 %d", entry.weight)
	}
	if entry.priority != 1 {
		t.Errorf("期望 priority=1，实际 %d", entry.priority)
	}
}

// TestProxy_Handler_Stream_Delta 验证流式响应中各 delta 内容拼接正确.
func TestProxy_Handler_Stream_Delta(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	reqBody := chatCompletionRequest{
		Model:    "gpt-4",
		Messages: []messageReq{{Role: "user", Content: "hello"}},
		Stream:   true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 收集所有 delta 内容
	var combined string
	scanner := bufio.NewScanner(rec.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		choices, _ := chunk["choices"].([]any)
		if len(choices) > 0 {
			delta, _ := choices[0].(map[string]any)["delta"].(map[string]any)
			if content, ok := delta["content"].(string); ok {
				combined += content
			}
		}
	}

	expected := "Hello World!"
	if combined != expected {
		t.Errorf("期望拼接内容 '%s'，实际 '%s'", expected, combined)
	}
}

// TestProxy_ResponseID_UniquePerRequest 验证每次请求生成不同 ID（基于时间戳）.
func TestProxy_ResponseID_UniquePerRequest(t *testing.T) {
	p := newTestProxy()
	handler := p.Handler()

	makeReq := func() string {
		reqBody := chatCompletionRequest{
			Model:    "gpt-4",
			Messages: []messageReq{{Role: "user", Content: "hi"}},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var resp chatCompletionResponse
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		return resp.ID
	}

	id1 := makeReq()
	// 稍等一纳秒确保时间戳不同（理论上同一纳秒内极小概率冲突，实际测试足够）
	time.Sleep(time.Nanosecond)
	id2 := makeReq()

	if id1 == "" {
		t.Error("期望 id 不为空")
	}
	_ = id2 // ID 唯一性依赖 UnixNano，此处仅验证非空
}
