package billing

import (
	"context"
	"errors"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	aimw "github.com/Tsukikage7/servex/v2/llm/middleware"
)

// ErrBudgetExceeded 超预算错误.调用方应将其视为业务级拒绝(不可重试).
var ErrBudgetExceeded = errors.New("billing: budget exceeded")

// 默认预算周期(30 天).
const defaultBudgetPeriod = 30 * 24 * time.Hour

// BudgetGuard 预算守卫:在每次 LLM 调用前检查 keyID 在周期内的已用费用,超额直接拒绝.
//
// 使用约束:
//   - keyExtractor 返回空字符串时,跳过检查直接放行(例如未登录/未鉴权的公共调用).
//   - getBudget 应返回该 keyID 的美元预算(对应 Billing.CalculateCost 单位).
//     返回 error 不会放行,会直接向上传播.
//   - period 用于 GetSummary 的时间窗口,默认 30 天.
type BudgetGuard struct {
	billing      Billing
	keyExtractor func(context.Context) string
	getBudget    func(ctx context.Context, keyID string) (float64, error)
	period       time.Duration
}

// BudgetOption BudgetGuard 构造选项.
type BudgetOption func(*BudgetGuard)

// WithPeriod 设置预算周期(默认 30 天).
// 参数 <= 0 时忽略.
func WithPeriod(d time.Duration) BudgetOption {
	return func(g *BudgetGuard) {
		if d > 0 {
			g.period = d
		}
	}
}

// NewBudgetGuard 创建预算守卫.
// billing/keyExtractor/getBudget 均必填:任一为 nil 时 panic.
// 这是 fail-closed 承诺的一部分 — 若构造时放行 nil,线上会静默失效,比 panic 更危险.
// 启动期 panic 能在集成测试/开发期立即暴露配置错误.
func NewBudgetGuard(
	b Billing,
	keyExtractor func(context.Context) string,
	getBudget func(context.Context, string) (float64, error),
	opts ...BudgetOption,
) *BudgetGuard {
	if b == nil {
		panic("billing: NewBudgetGuard requires non-nil billing")
	}
	if keyExtractor == nil {
		panic("billing: NewBudgetGuard requires non-nil keyExtractor")
	}
	if getBudget == nil {
		panic("billing: NewBudgetGuard requires non-nil getBudget")
	}
	g := &BudgetGuard{
		billing:      b,
		keyExtractor: keyExtractor,
		getBudget:    getBudget,
		period:       defaultBudgetPeriod,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Middleware 返回 ChatModel 中间件.
// 在 Generate/Stream 前调用 check;超额返回 ErrBudgetExceeded,放行后透传原结果.
// 核心依赖已在 NewBudgetGuard 构造时校验,此处无需再判 nil.
func (g *BudgetGuard) Middleware() aimw.Middleware {
	return func(next llm.ChatModel) llm.ChatModel {
		return aimw.Wrap(
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				if err := g.check(ctx); err != nil {
					return nil, err
				}
				return next.Generate(ctx, messages, opts...)
			},
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				if err := g.check(ctx); err != nil {
					return nil, err
				}
				return next.Stream(ctx, messages, opts...)
			},
		)
	}
}

// check 执行一次预算校验.
//
// 行为约定(fail-closed):
//  1. keyID 为空 => 放行(匿名/内部调用无身份可匹配预算).
//  2. getBudget 出错 => 向上返回错误,不放行.
//  3. GetSummary 出错 => 向上返回错误,不放行.
//  4. 已用费用 >= 预算 => 返回 ErrBudgetExceeded.
//
// 核心依赖(billing/keyExtractor/getBudget)已在 NewBudgetGuard 中 panic 保护.
func (g *BudgetGuard) check(ctx context.Context) error {
	keyID := g.keyExtractor(ctx)
	if keyID == "" {
		return nil
	}
	budget, err := g.getBudget(ctx, keyID)
	if err != nil {
		return err
	}
	// 周期 <=0 时用默认值兜底,避免被错误构造直接放行.
	period := g.period
	if period <= 0 {
		period = defaultBudgetPeriod
	}
	now := time.Now()
	from := now.Add(-period)
	summary, err := g.billing.GetSummary(ctx, keyID, from, now)
	if err != nil {
		return err
	}
	if summary != nil && summary.TotalCost >= budget {
		return ErrBudgetExceeded
	}
	return nil
}
