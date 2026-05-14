package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/serving/cache"
)

// ------------------------------------------------------------
// Mock 实现
// ------------------------------------------------------------

// mockEmbedding 模拟嵌入模型，按顺序循环返回预设向量.
type mockEmbedding struct {
	vectors [][]float32
	idx     int
}

func (m *mockEmbedding) EmbedTexts(_ context.Context, _ []string, _ ...llm.CallOption) (*llm.EmbedResponse, error) {
	v := m.vectors[m.idx%len(m.vectors)]
	m.idx++
	return &llm.EmbedResponse{Embeddings: [][]float32{v}}, nil
}

type emptyEmbedding struct{}

func (emptyEmbedding) EmbedTexts(context.Context, []string, ...llm.CallOption) (*llm.EmbedResponse, error) {
	return &llm.EmbedResponse{}, nil
}

// mockChat 模拟聊天模型，每次调用返回固定响应并计数.
type mockChat struct {
	response  *llm.ChatResponse
	callCount int
}

func (m *mockChat) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	m.callCount++
	return m.response, nil
}

func (m *mockChat) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	m.callCount++
	return nil, nil
}

// ------------------------------------------------------------
// MemoryStore 单元测试
// ------------------------------------------------------------

// TestMemoryStore_PutAndSearch 存入一条记录后用相同向量查询，应命中.
func TestMemoryStore_PutAndSearch(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryStore()
	ctx := t.Context()
	vec := []float32{1, 0, 0}
	resp := &llm.ChatResponse{Message: llm.AssistantMessage("hello")}

	if err := store.Put(ctx, vec, resp, time.Minute); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	got, err := store.Search(ctx, vec, 0.95)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if got == nil {
		t.Fatal("期望命中缓存，得到 nil")
	}
	if got.Message.Content != "hello" {
		t.Errorf("期望内容 'hello'，得到 %q", got.Message.Content)
	}
}

// TestMemoryStore_Threshold 当查询向量与存储向量相似度低于阈值时，应返回 nil（未命中）.
func TestMemoryStore_Threshold(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryStore()
	ctx := t.Context()

	// 存入 [1,0,0]
	if err := store.Put(ctx, []float32{1, 0, 0}, &llm.ChatResponse{Message: llm.AssistantMessage("a")}, time.Minute); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	// 用正交向量 [0,1,0] 查询，相似度为 0，远低于阈值 0.95
	got, err := store.Search(ctx, []float32{0, 1, 0}, 0.95)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if got != nil {
		t.Errorf("期望未命中，但得到 %+v", got)
	}
}

// TestMemoryStore_TTL 条目过期后应无法被检索到.
func TestMemoryStore_TTL(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryStore()
	ctx := t.Context()
	vec := []float32{1, 0, 0}
	resp := &llm.ChatResponse{Message: llm.AssistantMessage("ttl-test")}

	// 使用极短的 TTL
	if err := store.Put(ctx, vec, resp, 10*time.Millisecond); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	// 等待过期
	time.Sleep(20 * time.Millisecond)

	got, err := store.Search(ctx, vec, 0.95)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if got != nil {
		t.Error("期望条目已过期返回 nil，但得到命中结果")
	}
}

// TestMemoryStore_Clear 清空后查询应返回 nil.
func TestMemoryStore_Clear(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryStore()
	ctx := t.Context()
	vec := []float32{1, 0, 0}

	if err := store.Put(ctx, vec, &llm.ChatResponse{Message: llm.AssistantMessage("x")}, time.Minute); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Clear 失败: %v", err)
	}

	got, err := store.Search(ctx, vec, 0.95)
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	if got != nil {
		t.Error("Clear 后期望返回 nil，但命中了缓存")
	}
}

// ------------------------------------------------------------
// NewCachedModel 集成测试
// ------------------------------------------------------------

// TestNewCachedModel 验证第二次调用相同语义的消息时直接命中缓存，不再调用底层模型.
func TestNewCachedModel(t *testing.T) {
	t.Parallel()

	// 两次嵌入均返回相同向量，模拟语义完全相同的查询.
	sameVec := []float32{1, 0, 0}
	embed := &mockEmbedding{vectors: [][]float32{sameVec, sameVec}}

	chatResp := &llm.ChatResponse{
		Message:      llm.AssistantMessage("cached answer"),
		FinishReason: "stop",
	}
	chat := &mockChat{response: chatResp}

	model, err := cache.NewCachedModel(chat, &cache.Config{
		EmbeddingModel: embed,
		Store:          cache.NewMemoryStore(),
		Threshold:      0.95,
		TTL:            time.Minute,
	})
	if err != nil {
		t.Fatalf("NewCachedModel 失败: %v", err)
	}

	ctx := t.Context()
	msgs := []llm.Message{llm.UserMessage("你好")}

	// 第一次调用：缓存未命中，应透传到底层模型.
	resp1, err := model.Generate(ctx, msgs)
	if err != nil {
		t.Fatalf("第一次 Generate 失败: %v", err)
	}
	if resp1.Message.Content != "cached answer" {
		t.Errorf("期望 'cached answer'，得到 %q", resp1.Message.Content)
	}
	if chat.callCount != 1 {
		t.Errorf("期望底层模型被调用 1 次，实际 %d 次", chat.callCount)
	}

	// 第二次调用：应命中缓存，底层模型调用次数不增加.
	resp2, err := model.Generate(ctx, msgs)
	if err != nil {
		t.Fatalf("第二次 Generate 失败: %v", err)
	}
	if resp2.Message.Content != "cached answer" {
		t.Errorf("期望缓存命中 'cached answer'，得到 %q", resp2.Message.Content)
	}
	if chat.callCount != 1 {
		t.Errorf("缓存命中后底层模型调用次数应仍为 1，实际 %d 次", chat.callCount)
	}
}

func TestMiddlewareRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *cache.Config
		want error
	}{
		{name: "nil config", cfg: nil, want: cache.ErrNilConfig},
		{name: "nil embedding", cfg: &cache.Config{Store: cache.NewMemoryStore()}, want: cache.ErrNilEmbeddingModel},
		{name: "nil store", cfg: &cache.Config{EmbeddingModel: &mockEmbedding{vectors: [][]float32{{1}}}}, want: cache.ErrNilStore},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cache.Middleware(tc.cfg)
			if !errors.Is(err, tc.want) {
				t.Fatalf("期望 %v, 得到 %v", tc.want, err)
			}
		})
	}
}

func TestMiddlewareBypassesCacheWhenEmbeddingResponseIsEmpty(t *testing.T) {
	t.Parallel()

	mw, err := cache.Middleware(&cache.Config{
		EmbeddingModel: emptyEmbedding{},
		Store:          cache.NewMemoryStore(),
	})
	if err != nil {
		t.Fatalf("Middleware 失败: %v", err)
	}
	chat := &mockChat{response: &llm.ChatResponse{Message: llm.AssistantMessage("fallback")}}
	model := mw(chat)

	resp, err := model.Generate(t.Context(), []llm.Message{llm.UserMessage("你好")})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if resp.Message.Content != "fallback" {
		t.Fatalf("响应不匹配: %+v", resp)
	}
	if chat.callCount != 1 {
		t.Fatalf("空 embedding 应透传到底层模型, 调用次数 %d", chat.callCount)
	}
}
