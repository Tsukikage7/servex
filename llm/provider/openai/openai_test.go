package openai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/provider/openai"
)

// mockOpenAIServer 创建模拟 OpenAI 服务端.
func mockOpenAIServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestGenerate_Success(t *testing.T) {
	respBody := map[string]any{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"model":   "gpt-4o",
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "Hello!"}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
	}
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("意外路径: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("认证头错误: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	})

	client := openai.New("test-key", openai.WithBaseURL(srv.URL), openai.WithModel("gpt-4o"))
	resp, err := client.Generate(t.Context(), []llm.Message{llm.UserMessage("Hi")})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if resp.Message.Content != "Hello!" {
		t.Errorf("期望 'Hello!'，得到 %q", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("期望 TotalTokens=15，得到 %d", resp.Usage.TotalTokens)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("期望 FinishReason='stop'，得到 %q", resp.FinishReason)
	}
}

func TestGenerate_RateLimited(t *testing.T) {
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Rate limit exceeded", "type": "requests", "code": "rate_limit_exceeded"},
		})
	})

	client := openai.New("test-key", openai.WithBaseURL(srv.URL))
	_, err := client.Generate(t.Context(), []llm.Message{llm.UserMessage("Hi")})
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("期望限流错误，得到: %v", err)
	}
}

func TestGenerate_InvalidAuth(t *testing.T) {
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Invalid API key", "type": "invalid_request_error"},
		})
	})

	client := openai.New("wrong-key", openai.WithBaseURL(srv.URL))
	_, err := client.Generate(t.Context(), []llm.Message{llm.UserMessage("Hi")})
	if err == nil {
		t.Fatal("期望错误，得到 nil")
	}
}

func TestStream_SSEParsing(t *testing.T) {
	sseData := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null,\"index\":0}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\" World\"},\"finish_reason\":null,\"index\":0}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\",\"index\":0}]}\n\n" +
		"data: [DONE]\n\n"

	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	})

	client := openai.New("test-key", openai.WithBaseURL(srv.URL))
	reader, err := client.Stream(t.Context(), []llm.Message{llm.UserMessage("Hi")})
	if err != nil {
		t.Fatalf("Stream 失败: %v", err)
	}
	defer reader.Close()

	var fullContent string
	for {
		chunk, err := reader.Recv()
		if err != nil {
			break
		}
		fullContent += chunk.Delta
	}

	if fullContent != "Hello World" {
		t.Errorf("期望 'Hello World'，得到 %q", fullContent)
	}
}

func TestStream_AllowsLargeSSEChunk(t *testing.T) {
	largeDelta := strings.Repeat("x", 70*1024)
	sseData := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"" + largeDelta + "\"},\"finish_reason\":null,\"index\":0}]}\n\n" +
		"data: [DONE]\n\n"

	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	})

	client := openai.New("test-key", openai.WithBaseURL(srv.URL))
	reader, err := client.Stream(t.Context(), []llm.Message{llm.UserMessage("Hi")})
	if err != nil {
		t.Fatalf("Stream 失败: %v", err)
	}
	defer reader.Close()

	chunk, err := reader.Recv()
	if err != nil {
		t.Fatalf("Recv 失败: %v", err)
	}
	if chunk.Delta != largeDelta {
		t.Fatalf("期望读取完整长片段，长度=%d，得到长度=%d", len(largeDelta), len(chunk.Delta))
	}

	_, err = reader.Recv()
	if err != io.EOF {
		t.Fatalf("期望 EOF，得到 %v", err)
	}
}

func TestGenerate_WithTools(t *testing.T) {
	var receivedBody map[string]any
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-tc",
			"model": "gpt-4o",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []map[string]any{{
						"id":   "call_abc",
						"type": "function",
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": `{"location":"Beijing"}`,
						},
					}},
				},
				"finish_reason": "tool_calls",
			}},
			"usage": map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30},
		})
	})

	tool := llm.Tool{
		Function: llm.FunctionDef{
			Name:        "get_weather",
			Description: "获取天气信息",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
		},
	}

	client := openai.New("test-key", openai.WithBaseURL(srv.URL))
	resp, err := client.Generate(t.Context(),
		[]llm.Message{llm.UserMessage("北京天气怎么样？")},
		llm.WithTools(tool))
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	if len(resp.Message.ToolCalls) == 0 {
		t.Fatal("期望有工具调用，得到空")
	}
	if resp.Message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("期望工具名 'get_weather'，得到 %q", resp.Message.ToolCalls[0].Function.Name)
	}
	if resp.FinishReason != "tool_calls" {
		t.Errorf("期望 FinishReason='tool_calls'，得到 %q", resp.FinishReason)
	}
}

func TestEmbedTexts_Success(t *testing.T) {
	srv := mockOpenAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("意外路径: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"model":  "text-embedding-3-small",
			"data": []map[string]any{
				{"object": "embedding", "index": 0, "embedding": []float32{0.1, 0.2, 0.3}},
				{"object": "embedding", "index": 1, "embedding": []float32{0.4, 0.5, 0.6}},
			},
			"usage": map[string]any{"prompt_tokens": 4, "total_tokens": 4},
		})
	})

	client := openai.New("test-key", openai.WithBaseURL(srv.URL))
	resp, err := client.EmbedTexts(t.Context(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("EmbedTexts 失败: %v", err)
	}
	if len(resp.Embeddings) != 2 {
		t.Errorf("期望 2 个嵌入向量，得到 %d", len(resp.Embeddings))
	}
}

func TestGenerate_Integration(t *testing.T) {
	apiKey := os.Getenv("AI_OPENAI_KEY")
	if apiKey == "" {
		t.Skip("跳过集成测试：未设置 AI_OPENAI_KEY")
	}

	client := openai.New(apiKey)
	resp, err := client.Generate(t.Context(), []llm.Message{llm.UserMessage("说'你好'")},
		llm.WithMaxTokens(10))
	if err != nil {
		t.Fatalf("集成测试失败: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("期望非空响应")
	}
}

// isRateLimitError 检查错误是否为限流错误.
func isRateLimitError(err error) bool {
	return llm.IsRetryable(err)
}
