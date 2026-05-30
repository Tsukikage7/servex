package billing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/gateway/billing"
)

// --- budget 专用辅助 ---

// newFilledBilling 返回一个 MemoryStore-based Billing,并预置指定 keyID 的一条用量记录.
// cost 以美元计(PromptTokens=100 + OutputTokens=100 时价格等于 InputPrice+OutputPrice 再除以 10000).
// 为了直接可控,我们通过记录一次 usage,并靠定价让 CalculateCost 产生约等于 cost 的数值.
func newFilledBilling(t *testing.T, keyID string, cost float64) billing.Billing {
	t.Helper()
	store := billing.NewMemoryStore()
	// 使用特殊定价:input=cost*1e6,output=0,usage=(1 input token) → 费用 = cost.
	pricing := []billing.PriceModel{{
		ModelID:         "budget-test",
		InputPricePerM:  cost * 1_000_000, // 1 token 对应 cost 美元
		OutputPricePerM: 0,
	}}
	b, err := billing.NewBilling(store, billing.WithDefaultPricing(pricing))
	if err != nil {
		t.Fatalf("NewBilling: %v", err)
	}
	if err := b.Record(context.Background(), keyID, "budget-test", llm.Usage{PromptTokens: 1, TotalTokens: 1}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	return b
}

// --- 测试用例 ---

func TestBudgetGuard_AllowWhenUnderBudget(t *testing.T) {
	b := newFilledBilling(t, "k1", 1.0) // 已用 $1
	guard := billing.NewBudgetGuard(
		b,
		func(context.Context) string { return "k1" },
		func(context.Context, string) (float64, error) { return 10.0, nil }, // 预算 $10
	)

	model := &mockModel{}
	wrapped := guard.Middleware()(model)

	resp, err := wrapped.Generate(context.Background(), []llm.Message{llm.UserMessage("hi")})
	if err != nil {
		t.Fatalf("预期放行,得到 %v", err)
	}
	if resp == nil {
		t.Fatal("期望非空响应")
	}
}

func TestBudgetGuard_RejectWhenOverBudget(t *testing.T) {
	b := newFilledBilling(t, "k1", 20.0) // 已用 $20
	guard := billing.NewBudgetGuard(
		b,
		func(context.Context) string { return "k1" },
		func(context.Context, string) (float64, error) { return 10.0, nil }, // 预算 $10
	)

	model := &mockModel{}
	wrapped := guard.Middleware()(model)

	_, err := wrapped.Generate(context.Background(), []llm.Message{llm.UserMessage("hi")})
	if !errors.Is(err, billing.ErrBudgetExceeded) {
		t.Fatalf("期望 ErrBudgetExceeded,得到 %v", err)
	}
}

func TestBudgetGuard_EmptyKeyPassthrough(t *testing.T) {
	b := newFilledBilling(t, "k1", 100.0) // 大额已用
	getBudgetCalled := false
	guard := billing.NewBudgetGuard(
		b,
		func(context.Context) string { return "" }, // 空 key
		func(context.Context, string) (float64, error) {
			getBudgetCalled = true
			return 0, nil
		},
	)

	model := &mockModel{}
	wrapped := guard.Middleware()(model)
	if _, err := wrapped.Generate(context.Background(), []llm.Message{llm.UserMessage("hi")}); err != nil {
		t.Fatalf("期望放行,得到 %v", err)
	}
	if getBudgetCalled {
		t.Error("空 keyID 时不应调用 getBudget")
	}
}

func TestBudgetGuard_GetBudgetErrorPropagates(t *testing.T) {
	b := newFilledBilling(t, "k1", 1.0)
	targetErr := errors.New("lookup failed")
	guard := billing.NewBudgetGuard(
		b,
		func(context.Context) string { return "k1" },
		func(context.Context, string) (float64, error) { return 0, targetErr },
	)
	wrapped := guard.Middleware()(&mockModel{})
	_, err := wrapped.Generate(context.Background(), []llm.Message{llm.UserMessage("hi")})
	if !errors.Is(err, targetErr) {
		t.Fatalf("期望 targetErr 透传,得到 %v", err)
	}
}

func TestBudgetGuard_StreamAlsoBlocked(t *testing.T) {
	b := newFilledBilling(t, "k1", 20.0)
	guard := billing.NewBudgetGuard(
		b,
		func(context.Context) string { return "k1" },
		func(context.Context, string) (float64, error) { return 10.0, nil },
	)
	wrapped := guard.Middleware()(&mockModel{})
	_, err := wrapped.Stream(context.Background(), []llm.Message{llm.UserMessage("hi")})
	if !errors.Is(err, billing.ErrBudgetExceeded) {
		t.Fatalf("期望 ErrBudgetExceeded,得到 %v", err)
	}
}

func TestBudgetGuard_WithPeriod(t *testing.T) {
	// 自定义 period 不影响本测试的 memory store(无时间过滤效果),仅验证构造不报错.
	b := newFilledBilling(t, "k1", 1.0)
	guard := billing.NewBudgetGuard(
		b,
		func(context.Context) string { return "k1" },
		func(context.Context, string) (float64, error) { return 5.0, nil },
		billing.WithPeriod(7*24*time.Hour),
	)
	wrapped := guard.Middleware()(&mockModel{})
	if _, err := wrapped.Generate(context.Background(), nil); err != nil {
		t.Fatalf("期望放行,得到 %v", err)
	}
}

// nil 核心依赖必须在构造阶段 fail-fast.
func TestBudgetGuard_NewPanicsOnNilDeps(t *testing.T) {
	b, err := billing.NewBilling(billing.NewMemoryStore())
	if err != nil {
		t.Fatalf("NewBilling: %v", err)
	}
	nonNilKey := func(context.Context) string { return "k" }
	nonNilGet := func(context.Context, string) (float64, error) { return 1, nil }

	cases := []struct {
		name      string
		mkBilling billing.Billing
		key       func(context.Context) string
		get       func(context.Context, string) (float64, error)
	}{
		{"nil billing", nil, nonNilKey, nonNilGet},
		{"nil keyExtractor", b, nil, nonNilGet},
		{"nil getBudget", b, nonNilKey, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("%s: 期望 panic,但没有", tc.name)
				}
			}()
			_ = billing.NewBudgetGuard(tc.mkBilling, tc.key, tc.get)
		})
	}
}
