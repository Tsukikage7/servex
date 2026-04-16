package deepseek_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/provider/deepseek"
)

func TestNew_DefaultConfig(t *testing.T) {
	client := deepseek.New("sk-test")
	if client == nil {
		t.Fatal("New 返回 nil")
	}

	// 编译期接口断言已在包内声明，此处再做运行时验证
	var _ llm.ChatModel = client
	var _ llm.EmbeddingModel = client
}

func TestNew_WithOptions(t *testing.T) {
	customURL := "https://custom.deepseek.example.com/v1"
	customModel := "deepseek-reasoner"
	customEmbedModel := "deepseek-embedding"
	customHTTPClient := &http.Client{Timeout: 90 * time.Second}

	client := deepseek.New("sk-test",
		deepseek.WithBaseURL(customURL),
		deepseek.WithModel(customModel),
		deepseek.WithEmbeddingModel(customEmbedModel),
		deepseek.WithHTTPClient(customHTTPClient),
	)
	if client == nil {
		t.Fatal("New 返回 nil")
	}

	var _ llm.ChatModel = client
	var _ llm.EmbeddingModel = client
}

func TestNew_InterfaceAssertions(t *testing.T) {
	client := deepseek.New("sk-test")

	if _, ok := any(client).(llm.ChatModel); !ok {
		t.Error("Client 未实现 llm.ChatModel 接口")
	}
	if _, ok := any(client).(llm.EmbeddingModel); !ok {
		t.Error("Client 未实现 llm.EmbeddingModel 接口")
	}
}

func TestGenerate_WithMockServer(t *testing.T) {
	respBody := map[string]any{
		"id":      "chatcmpl-ds-123",
		"object":  "chat.completion",
		"model":   "deepseek-chat",
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "你好！"}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))
	t.Cleanup(srv.Close)

	client := deepseek.New("sk-test",
		deepseek.WithBaseURL(srv.URL),
	)
	resp, err := client.Generate(t.Context(), []llm.Message{llm.UserMessage("你好")})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if resp.Message.Content != "你好！" {
		t.Errorf("期望 '你好！'，得到 %q", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("期望 TotalTokens=8，得到 %d", resp.Usage.TotalTokens)
	}
}

func TestGenerate_Integration(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("跳过集成测试：未设置 DEEPSEEK_API_KEY")
	}

	client := deepseek.New(apiKey)
	resp, err := client.Generate(t.Context(), []llm.Message{llm.UserMessage("说'你好'")},
		llm.WithMaxTokens(10))
	if err != nil {
		t.Fatalf("集成测试失败: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("期望非空响应")
	}
}
