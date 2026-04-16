package ollama_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/provider/ollama"
)

func TestNew_DefaultConfig(t *testing.T) {
	client := ollama.New("")
	if client == nil {
		t.Fatal("New 返回 nil")
	}

	// 编译期接口断言已在包内声明，此处再做运行时验证
	var _ llm.ChatModel = client
	var _ llm.EmbeddingModel = client
}

func TestNew_WithOptions(t *testing.T) {
	customURL := "http://192.168.1.100:11434/v1"
	customModel := "qwen2.5:7b"
	customEmbedModel := "nomic-embed-text"
	customHTTPClient := &http.Client{Timeout: 30 * time.Second}

	client := ollama.New("",
		ollama.WithBaseURL(customURL),
		ollama.WithModel(customModel),
		ollama.WithEmbeddingModel(customEmbedModel),
		ollama.WithHTTPClient(customHTTPClient),
	)
	if client == nil {
		t.Fatal("New 返回 nil")
	}

	// 接口兼容性验证
	var _ llm.ChatModel = client
	var _ llm.EmbeddingModel = client
}

func TestNew_InterfaceAssertions(t *testing.T) {
	// 验证 Client 同时满足两个接口
	client := ollama.New("")

	if _, ok := any(client).(llm.ChatModel); !ok {
		t.Error("Client 未实现 llm.ChatModel 接口")
	}
	if _, ok := any(client).(llm.EmbeddingModel); !ok {
		t.Error("Client 未实现 llm.EmbeddingModel 接口")
	}
}
