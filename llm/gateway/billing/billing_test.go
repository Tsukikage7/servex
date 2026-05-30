package billing_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/gateway/billing"
)

// --- 测试辅助 ---

// testPricing 返回用于测试的标准定价配置.
func testPricing() []billing.PriceModel {
	return []billing.PriceModel{
		{
			ModelID:         "gpt-4o",
			InputPricePerM:  5.0,
			OutputPricePerM: 15.0,
			CachedPricePerM: 2.5,
		},
		{
			ModelID:         "gpt-3.5-turbo",
			InputPricePerM:  0.5,
			OutputPricePerM: 1.5,
			CachedPricePerM: 0.25,
		},
	}
}

// newTestBilling 创建使用 MemoryStore 的计费引擎.
func newTestBilling(t *testing.T) billing.Billing {
	t.Helper()
	store := billing.NewMemoryStore()
	b, err := billing.NewBilling(store, billing.WithDefaultPricing(testPricing()))
	if err != nil {
		t.Fatalf("NewBilling 失败: %v", err)
	}
	return b
}

// mockModel 用于测试的模拟 ChatModel.
type mockModel struct {
	generateFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error)
}

func (m *mockModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, messages, opts...)
	}
	return &llm.ChatResponse{
		Message: llm.AssistantMessage("ok"),
		ModelID: "gpt-4o",
		Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
	}, nil
}

func (m *mockModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return &mockReader{
		resp: &llm.ChatResponse{
			Message: llm.AssistantMessage("ok"),
			ModelID: "gpt-4o",
			Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		},
	}, nil
}

// mockReader 模拟 StreamReader.
type mockReader struct {
	resp *llm.ChatResponse
	sent bool
}

func (r *mockReader) Recv() (llm.StreamChunk, error) {
	if r.sent {
		return llm.StreamChunk{}, io.EOF
	}
	r.sent = true
	return llm.StreamChunk{Delta: "ok", FinishReason: "stop"}, nil
}

func (r *mockReader) Response() *llm.ChatResponse { return r.resp }
func (r *mockReader) Close() error                { return nil }

type failingBilling struct {
	err error
}

func (b failingBilling) Record(context.Context, string, string, llm.Usage) error {
	return b.err
}

func (b failingBilling) GetSummary(context.Context, string, time.Time, time.Time) (*billing.Summary, error) {
	return nil, nil
}

func (b failingBilling) SetPricing(string, billing.PriceModel) {}

func (b failingBilling) CalculateCost(string, llm.Usage) float64 { return 0 }

// --- 测试用例 ---

// TestCalculateCost 验证费用计算逻辑.
func TestCalculateCost(t *testing.T) {
	b := newTestBilling(t)

	usage := llm.Usage{
		PromptTokens:     1_000_000, // 100 万输入 token
		CompletionTokens: 500_000,   // 50 万输出 token
		CachedTokens:     200_000,   // 20 万缓存命中 token
		TotalTokens:      1_700_000,
	}

	// gpt-4o: input=5.0, output=15.0, cached=2.5每百万 token
	// 期望费用 = (1_000_000 * 5 + 500_000 * 15 + 200_000 * 2.5) / 1_000_000
	//          = (5_000_000 + 7_500_000 + 500_000) / 1_000_000
	//          = 13_000_000 / 1_000_000 = 13.0
	const wantCost = 13.0

	got := b.CalculateCost("gpt-4o", usage)
	if got != wantCost {
		t.Errorf("CalculateCost 结果错误: got %.6f, want %.6f", got, wantCost)
	}
}

// TestCalculateCost_UnknownModel 验证未知模型的费用为 0.
func TestCalculateCost_UnknownModel(t *testing.T) {
	b := newTestBilling(t)

	usage := llm.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	got := b.CalculateCost("unknown-model", usage)
	if got != 0 {
		t.Errorf("未知模型费用应为 0, got %.6f", got)
	}
}

// TestRecord 验证用量记录功能.
func TestRecord(t *testing.T) {
	store := billing.NewMemoryStore()
	b, err := billing.NewBilling(store, billing.WithDefaultPricing(testPricing()))
	if err != nil {
		t.Fatalf("NewBilling 失败: %v", err)
	}

	ctx := t.Context()
	usage := llm.Usage{
		PromptTokens:     500,
		CompletionTokens: 200,
		TotalTokens:      700,
	}

	if err := b.Record(ctx, "key-001", "gpt-4o", usage); err != nil {
		t.Fatalf("Record 失败: %v", err)
	}

	// 通过 GetSummary 验证记录已存储
	from := time.Now().Add(-time.Minute)
	to := time.Now().Add(time.Minute)

	summary, err := b.GetSummary(ctx, "key-001", from, to)
	if err != nil {
		t.Fatalf("GetSummary 失败: %v", err)
	}

	if summary.TotalRequests != 1 {
		t.Errorf("TotalRequests 应为 1, got %d", summary.TotalRequests)
	}
	if summary.TotalTokens != 700 {
		t.Errorf("TotalTokens 应为 700, got %d", summary.TotalTokens)
	}

	// 验证费用已计算
	// gpt-4o: (500 * 5 + 200 * 15 + 0 * 2.5) / 1_000_000 = 5500 / 1_000_000 = 0.0055
	const wantCost = 0.0055
	if summary.TotalCost != wantCost {
		t.Errorf("TotalCost 应为 %.6f, got %.6f", wantCost, summary.TotalCost)
	}
}

// TestGetSummary 验证多条记录的汇总聚合逻辑.
func TestGetSummary(t *testing.T) {
	store := billing.NewMemoryStore()
	b, err := billing.NewBilling(store, billing.WithDefaultPricing(testPricing()))
	if err != nil {
		t.Fatalf("NewBilling 失败: %v", err)
	}

	ctx := t.Context()
	keyID := "key-agg"

	// 记录 3 条：gpt-4o × 2，gpt-3.5-turbo × 1
	records := []struct {
		modelID string
		usage   llm.Usage
	}{
		{
			modelID: "gpt-4o",
			usage:   llm.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500},
		},
		{
			modelID: "gpt-4o",
			usage:   llm.Usage{PromptTokens: 2000, CompletionTokens: 1000, TotalTokens: 3000},
		},
		{
			modelID: "gpt-3.5-turbo",
			usage:   llm.Usage{PromptTokens: 500, CompletionTokens: 200, TotalTokens: 700},
		},
	}

	for _, r := range records {
		if err := b.Record(ctx, keyID, r.modelID, r.usage); err != nil {
			t.Fatalf("Record 失败: %v", err)
		}
	}

	from := time.Now().Add(-time.Minute)
	to := time.Now().Add(time.Minute)

	summary, err := b.GetSummary(ctx, keyID, from, to)
	if err != nil {
		t.Fatalf("GetSummary 失败: %v", err)
	}

	// 验证总请求数
	if summary.TotalRequests != 3 {
		t.Errorf("TotalRequests 应为 3, got %d", summary.TotalRequests)
	}

	// 验证总 token 数：1500 + 3000 + 700 = 5200
	if summary.TotalTokens != 5200 {
		t.Errorf("TotalTokens 应为 5200, got %d", summary.TotalTokens)
	}

	// 验证 ByModel 按模型分组
	if len(summary.ByModel) != 2 {
		t.Errorf("ByModel 应有 2 个模型, got %d", len(summary.ByModel))
	}

	gpt4o, ok := summary.ByModel["gpt-4o"]
	if !ok {
		t.Fatal("ByModel 中应包含 gpt-4o")
	}
	if gpt4o.Requests != 2 {
		t.Errorf("gpt-4o 请求数应为 2, got %d", gpt4o.Requests)
	}
	if gpt4o.Tokens != 4500 {
		t.Errorf("gpt-4o token 数应为 4500, got %d", gpt4o.Tokens)
	}

	gpt35, ok := summary.ByModel["gpt-3.5-turbo"]
	if !ok {
		t.Fatal("ByModel 中应包含 gpt-3.5-turbo")
	}
	if gpt35.Requests != 1 {
		t.Errorf("gpt-3.5-turbo 请求数应为 1, got %d", gpt35.Requests)
	}
	if gpt35.Tokens != 700 {
		t.Errorf("gpt-3.5-turbo token 数应为 700, got %d", gpt35.Tokens)
	}
}

// TestSetPricing 验证定价修改后费用计算的变化.
func TestSetPricing(t *testing.T) {
	b := newTestBilling(t)

	usage := llm.Usage{
		PromptTokens:     1_000_000,
		CompletionTokens: 0,
		TotalTokens:      1_000_000,
	}

	// 使用初始 gpt-4o 定价：input=5.0，期望费用 = 5.0
	costBefore := b.CalculateCost("gpt-4o", usage)
	if costBefore != 5.0 {
		t.Errorf("修改前费用应为 5.0, got %.6f", costBefore)
	}

	// 修改定价：input 改为 10.0
	b.SetPricing("gpt-4o", billing.PriceModel{
		ModelID:         "gpt-4o",
		InputPricePerM:  10.0,
		OutputPricePerM: 30.0,
		CachedPricePerM: 5.0,
	})

	// 修改后期望费用 = (1_000_000 * 10) / 1_000_000 = 10.0
	costAfter := b.CalculateCost("gpt-4o", usage)
	if costAfter != 10.0 {
		t.Errorf("修改后费用应为 10.0, got %.6f", costAfter)
	}
}

// TestMiddleware 验证计费中间件在 Generate 调用后自动记录用量.
func TestMiddleware(t *testing.T) {
	store := billing.NewMemoryStore()
	b, err := billing.NewBilling(store, billing.WithDefaultPricing(testPricing()))
	if err != nil {
		t.Fatalf("NewBilling 失败: %v", err)
	}

	// keyExtractor 返回固定的 key ID
	const keyID = "key-mw-test"
	keyExtractor := func(_ context.Context) string { return keyID }

	// 模拟模型，返回固定用量
	model := &mockModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{
				Message: llm.AssistantMessage("hello"),
				ModelID: "gpt-4o",
				Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			}, nil
		},
	}

	// 应用计费中间件
	mw := billing.Middleware(b, keyExtractor)
	wrapped := mw(model)

	ctx := t.Context()
	_, err = wrapped.Generate(ctx, []llm.Message{llm.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	// 验证计费记录已存储
	from := time.Now().Add(-time.Minute)
	to := time.Now().Add(time.Minute)

	summary, err := b.GetSummary(ctx, keyID, from, to)
	if err != nil {
		t.Fatalf("GetSummary 失败: %v", err)
	}

	if summary.TotalRequests != 1 {
		t.Errorf("中间件应记录 1 次请求, got %d", summary.TotalRequests)
	}
	if summary.TotalTokens != 150 {
		t.Errorf("TotalTokens 应为 150, got %d", summary.TotalTokens)
	}
	if summary.TotalCost <= 0 {
		t.Errorf("TotalCost 应大于 0, got %.6f", summary.TotalCost)
	}

	// 验证未记录到其他 key
	summaryOther, err := b.GetSummary(ctx, "other-key", from, to)
	if err != nil {
		t.Fatalf("GetSummary (other-key) 失败: %v", err)
	}
	if summaryOther.TotalRequests != 0 {
		t.Errorf("other-key 不应有记录, got %d", summaryOther.TotalRequests)
	}
}

func TestNewBillingRejectsNilStore(t *testing.T) {
	t.Parallel()

	_, err := billing.NewBilling(nil)
	if !errors.Is(err, billing.ErrNilStore) {
		t.Fatalf("期望 ErrNilStore, 得到 %v", err)
	}
}

func TestMiddlewareStreamReturnsRecordError(t *testing.T) {
	t.Parallel()

	targetErr := errors.New("record failed")
	wrapped := billing.Middleware(failingBilling{err: targetErr}, func(context.Context) string { return "key" })(&mockModel{})
	reader, err := wrapped.Stream(t.Context(), []llm.Message{llm.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Stream 失败: %v", err)
	}
	defer reader.Close()

	if _, err := reader.Recv(); err != nil {
		t.Fatalf("第一次 Recv 失败: %v", err)
	}
	_, err = reader.Recv()
	if !errors.Is(err, targetErr) {
		t.Fatalf("期望记录错误, 得到 %v", err)
	}
}
