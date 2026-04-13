// Package billing 提供 AI 服务的用量计费功能.
package billing

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/llm"
	aimw "github.com/Tsukikage7/servex/v2/llm/middleware"
)

// 预定义错误.
var (
	// ErrNilStore Store 为 nil 时返回.
	ErrNilStore = errors.New("billing: store is nil")
)

// PriceModel 定价模型，描述某个模型的 token 单价.
type PriceModel struct {
	ModelID         string  `json:"model_id"`
	InputPricePerM  float64 `json:"input_price_per_m"`  // 每 100 万输入 token 的价格
	OutputPricePerM float64 `json:"output_price_per_m"` // 每 100 万输出 token 的价格
	CachedPricePerM float64 `json:"cached_price_per_m"` // 每 100 万缓存命中 token 的价格
}

// UsageRecord 单次请求的用量记录.
type UsageRecord struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	KeyID     string    `json:"key_id" gorm:"index"`
	ModelID   string    `json:"model_id"`
	Usage     llm.Usage `json:"usage" gorm:"serializer:json"`
	Cost      float64   `json:"cost"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 返回 GORM 表名.
func (UsageRecord) TableName() string { return "usage_records" }

// Summary 汇总统计结果.
type Summary struct {
	TotalRequests int64                   `json:"total_requests"`
	TotalTokens   int64                   `json:"total_tokens"`
	TotalCost     float64                 `json:"total_cost"`
	ByModel       map[string]ModelSummary `json:"by_model,omitzero"`
}

// ModelSummary 按模型维度的统计结果.
type ModelSummary struct {
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"total_tokens"`
	Cost     float64 `json:"cost"`
}

// Billing 计费引擎接口.
type Billing interface {
	// Record 记录一次 AI 请求的用量，并持久化到存储.
	Record(ctx context.Context, keyID string, modelID string, usage llm.Usage) error
	// GetSummary 查询指定 keyID 在时间范围内的用量汇总.
	GetSummary(ctx context.Context, keyID string, from, to time.Time) (*Summary, error)
	// SetPricing 设置或更新某个模型的定价.
	SetPricing(modelID string, pricing PriceModel)
	// CalculateCost 根据定价和用量计算费用.
	CalculateCost(modelID string, usage llm.Usage) float64
}

// Store 存储接口.
type Store interface {
	// SaveRecord 保存一条用量记录.
	SaveRecord(ctx context.Context, record *UsageRecord) error
	// GetRecords 查询指定 keyID 在时间范围内的所有用量记录.
	GetRecords(ctx context.Context, keyID string, from, to time.Time) ([]UsageRecord, error)
	// AutoMigrate 自动迁移数据库表结构.
	AutoMigrate(ctx context.Context) error
}

// --- Memory Store ---

// memoryStore 基于内存的 Store 实现，用于测试.
type memoryStore struct {
	mu      sync.RWMutex
	records []UsageRecord
}

// NewMemoryStore 创建基于内存的 Store，用于测试.
func NewMemoryStore() Store {
	return &memoryStore{}
}

func (s *memoryStore) SaveRecord(_ context.Context, record *UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := *record
	s.records = append(s.records, copied)
	return nil
}

func (s *memoryStore) GetRecords(_ context.Context, keyID string, from, to time.Time) ([]UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []UsageRecord
	for _, r := range s.records {
		if r.KeyID == keyID && !r.CreatedAt.Before(from) && !r.CreatedAt.After(to) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *memoryStore) AutoMigrate(_ context.Context) error {
	return nil
}

// --- Billing 实现 ---

// Option Billing 构造选项.
type Option func(*billingImpl)

// WithDefaultPricing 设置初始定价列表.
func WithDefaultPricing(models []PriceModel) Option {
	return func(b *billingImpl) {
		for _, m := range models {
			b.pricing[m.ModelID] = m
		}
	}
}

// billingImpl Billing 接口的默认实现.
type billingImpl struct {
	store   Store
	mu      sync.RWMutex
	pricing map[string]PriceModel
}

// NewBilling 创建计费引擎实例.
func NewBilling(store Store, opts ...Option) Billing {
	b := &billingImpl{
		store:   store,
		pricing: make(map[string]PriceModel),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// CalculateCost 根据定价和 token 用量计算费用.
// 公式：(inputTokens * inputPrice + outputTokens * outputPrice + cachedTokens * cachedPrice) / 1_000_000
func (b *billingImpl) CalculateCost(modelID string, usage llm.Usage) float64 {
	b.mu.RLock()
	pm, ok := b.pricing[modelID]
	b.mu.RUnlock()
	if !ok {
		return 0
	}
	return (float64(usage.PromptTokens)*pm.InputPricePerM +
		float64(usage.CompletionTokens)*pm.OutputPricePerM +
		float64(usage.CachedTokens)*pm.CachedPricePerM) / 1_000_000
}

// Record 计算费用并将用量记录保存到存储.
func (b *billingImpl) Record(ctx context.Context, keyID string, modelID string, usage llm.Usage) error {
	cost := b.CalculateCost(modelID, usage)
	record := &UsageRecord{
		ID:        uuid.New().String(),
		KeyID:     keyID,
		ModelID:   modelID,
		Usage:     usage,
		Cost:      cost,
		CreatedAt: time.Now(), // GORM autoCreateTime 仅在数据库层生效，此处显式赋值保证内存 Store 正确过滤
	}
	return b.store.SaveRecord(ctx, record)
}

// GetSummary 查询指定 keyID 在时间范围内的汇总统计.
func (b *billingImpl) GetSummary(ctx context.Context, keyID string, from, to time.Time) (*Summary, error) {
	records, err := b.store.GetRecords(ctx, keyID, from, to)
	if err != nil {
		return nil, err
	}

	summary := &Summary{
		ByModel: make(map[string]ModelSummary),
	}

	for _, r := range records {
		summary.TotalRequests++
		summary.TotalTokens += int64(r.Usage.TotalTokens)
		summary.TotalCost += r.Cost

		ms := summary.ByModel[r.ModelID]
		ms.Requests++
		ms.Tokens += int64(r.Usage.TotalTokens)
		ms.Cost += r.Cost
		summary.ByModel[r.ModelID] = ms
	}

	return summary, nil
}

// SetPricing 设置或更新某个模型的定价.
func (b *billingImpl) SetPricing(modelID string, pricing PriceModel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pricing[modelID] = pricing
}

// Middleware 返回计费中间件.
// keyExtractor 从 context 中提取 API key ID（配合 ai/apikey 使用）.
func Middleware(b Billing, keyExtractor func(ctx context.Context) string) aimw.Middleware {
	return func(next llm.ChatModel) llm.ChatModel {
		return aimw.Wrap(
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				resp, err := next.Generate(ctx, messages, opts...)
				if err == nil && resp != nil {
					keyID := keyExtractor(ctx)
					// 异步记录不阻塞主流程，错误静默忽略
					_ = b.Record(ctx, keyID, resp.ModelID, resp.Usage)
				}
				return resp, err
			},
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				// 流式接口：包装 StreamReader，在流结束后记录用量
				reader, err := next.Stream(ctx, messages, opts...)
				if err != nil {
					return nil, err
				}
				return &billingStreamReader{
					reader:       reader,
					billing:      b,
					ctx:          context.WithoutCancel(ctx),
					keyExtractor: keyExtractor,
				}, nil
			},
		)
	}
}

// billingStreamReader 在流结束后记录用量的 StreamReader 包装器.
type billingStreamReader struct {
	reader       llm.StreamReader
	billing      Billing
	ctx          context.Context //nolint:containedctx // 使用 WithoutCancel 避免生命周期问题
	keyExtractor func(ctx context.Context) string
	done         bool
}

// Recv 读取下一个片段，流结束时记录用量.
func (r *billingStreamReader) Recv() (llm.StreamChunk, error) {
	chunk, err := r.reader.Recv()
	if err != nil && !r.done {
		r.done = true
		if resp := r.reader.Response(); resp != nil {
			keyID := r.keyExtractor(r.ctx)
			_ = r.billing.Record(r.ctx, keyID, resp.ModelID, resp.Usage)
		}
	}
	return chunk, err
}

// Response 获取完整响应.
func (r *billingStreamReader) Response() *llm.ChatResponse {
	return r.reader.Response()
}

// Close 关闭流.
func (r *billingStreamReader) Close() error {
	return r.reader.Close()
}

// 编译期接口断言.
var _ llm.StreamReader = (*billingStreamReader)(nil)
