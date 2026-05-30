# llm/gateway/billing

`github.com/Tsukikage7/servex/v2/llm/gateway/billing` — AI 服务用量计费，支持按模型定价、用量记录、汇总统计及计费中间件。

## 核心类型

- `Billing` — 计费引擎接口，方法包括 Record、GetSummary、SetPricing、CalculateCost
- `PriceModel` — 定价模型，包含 ModelID、InputPricePerM、OutputPricePerM、CachedPricePerM（每百万 token 价格）
- `UsageRecord` — 单次请求用量记录，包含 KeyID、ModelID、Usage、Cost、CreatedAt
- `Summary` — 汇总统计，包含 TotalRequests、TotalTokens、TotalCost、ByModel
- `Store` — 存储接口，方法包括 SaveRecord、GetRecords、AutoMigrate
- `NewBilling(store, opts...)` — 校验存储并创建计费引擎
- `NewGORMStore(db)` — 基于 GORM 的持久化存储
- `NewMemoryStore()` — 基于内存的存储（用于测试）
- `Middleware(b, keyExtractor)` — 计费中间件，在 Generate/Stream 后自动记录用量

## 使用示例

```go
import "github.com/Tsukikage7/servex/v2/llm/gateway/billing"

store := billing.NewGORMStore(db)
b, err := billing.NewBilling(store,
    billing.WithDefaultPricing([]billing.PriceModel{
        {ModelID: "gpt-4o", InputPricePerM: 2.5, OutputPricePerM: 10.0},
    }),
)
if err != nil {
    return err
}

// 作为中间件
billingMw := billing.Middleware(b, func(ctx context.Context) string {
    key, _ := apikey.FromContext(ctx)
    if key != nil { return key.ID }
    return ""
})
billedModel := billingMw(myModel)

// 查询汇总
summary, _ := b.GetSummary(ctx, "key-123",
    time.Now().AddDate(0, -1, 0), time.Now())
fmt.Printf("总费用: $%.4f，总 tokens: %d\n",
    summary.TotalCost, summary.TotalTokens)
```

## BudgetGuard（预算熔断中间件）

`BudgetGuard` 在每次 LLM 调用**之前**查询该 API Key 在指定周期内的已用费用，若超过预算则拒绝调用，返回 `billing.ErrBudgetExceeded`。

```go
import "github.com/Tsukikage7/servex/v2/llm/gateway/billing"

guard := billing.NewBudgetGuard(
    b,
    func(ctx context.Context) string {
        key, _ := apikey.FromContext(ctx)
        if key != nil { return key.ID }
        return ""
    },
    func(ctx context.Context, keyID string) (float64, error) {
        // 从你的业务配置/数据库读取该 key 的美元预算
        return budgetStore.Get(ctx, keyID)
    },
    billing.WithPeriod(30*24*time.Hour), // 默认 30 天
)

guardedModel := guard.Middleware()(myModel)

resp, err := guardedModel.Generate(ctx, messages)
if errors.Is(err, billing.ErrBudgetExceeded) {
    // 返回 429/402 等,提示用户升级
}
```

语义要点（**fail-closed**）：
- `NewBudgetGuard` 的 `billing` / `keyExtractor` / `getBudget` 任一为 `nil` 时**构造即 panic**（启动期暴露配置错误，避免线上静默放行）。
- `keyExtractor` 返回空串：跳过检查（例如匿名/内部调用）。
- `getBudget` 或 `GetSummary` 返回错误：错误向上传递，**不会放行**。
- 已用 `>= 预算` 即拒绝（边界包含在内）。
- 可与 `Middleware(b, keyExtractor)` 串联：`BudgetGuard` 做熔断、计费中间件做记录。建议把 `BudgetGuard` 放在**外层**。
