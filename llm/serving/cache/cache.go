// Package cache 提供基于语义相似度的 AI 响应缓存.
//
// 通过将用户消息嵌入为向量，并与已缓存的向量做余弦相似度比较，
// 当相似度超过阈值时直接返回缓存结果，避免重复调用语言模型.
package cache

import (
	"context"
	"errors"
	"io"
	"math"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/middleware"
)

// Config 语义缓存配置.
type Config struct {
	// EmbeddingModel 用于将文本转换为向量的嵌入模型.
	EmbeddingModel llm.EmbeddingModel
	// Store 缓存存储后端.
	Store Store
	// Threshold 相似度阈值，超过此值视为缓存命中，默认 0.95.
	Threshold float32
	// TTL 缓存条目的存活时长，默认 1h.
	TTL time.Duration
}

// withDefaults 填充 Config 中未设置的默认值.
func (c *Config) withDefaults() *Config {
	cfg := *c
	if cfg.Threshold == 0 {
		cfg.Threshold = 0.95
	}
	if cfg.TTL == 0 {
		cfg.TTL = time.Hour
	}
	return &cfg
}

// Store 缓存存储接口.
type Store interface {
	// Put 存入一条缓存条目，key 为查询向量，value 为对应响应.
	Put(ctx context.Context, key []float32, value *llm.ChatResponse, ttl time.Duration) error
	// Search 在存储中查找与 query 最相似且相似度不低于 threshold 的缓存响应.
	// 未命中时返回 nil, nil.
	Search(ctx context.Context, query []float32, threshold float32) (*llm.ChatResponse, error)
	// Clear 清空所有缓存条目.
	Clear(ctx context.Context) error
}

// entry 单条缓存记录.
type entry struct {
	// vector 该条目的查询向量.
	vector []float32
	// response 对应的聊天响应.
	response *llm.ChatResponse
	// expireAt 过期时间.
	expireAt time.Time
}

// MemoryStore 基于内存切片的缓存实现，线性扫描余弦相似度.
// 注意：Search 方法对所有条目执行线性扫描（O(n)），仅适用于条目数较少的场景（建议 < 10000 条）.
// 生产环境中若缓存条目较多，建议使用基于向量数据库的 Store 实现.
type MemoryStore struct {
	mu      sync.RWMutex
	entries []entry
}

// NewMemoryStore 创建一个新的内存缓存存储.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Put 向内存存储中追加一条缓存条目，并顺便清理已过期的条目.
func (s *MemoryStore) Put(_ context.Context, key []float32, value *llm.ChatResponse, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	// 清理已过期条目.
	active := s.entries[:0]
	for _, e := range s.entries {
		if e.expireAt.After(now) {
			active = append(active, e)
		}
	}
	s.entries = active

	s.entries = append(s.entries, entry{
		vector:   key,
		response: value,
		expireAt: now.Add(ttl),
	})
	return nil
}

// Search 遍历所有未过期条目，计算余弦相似度，返回相似度最高且不低于 threshold 的响应.
// 未找到时返回 nil, nil.
func (s *MemoryStore) Search(_ context.Context, query []float32, threshold float32) (*llm.ChatResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var (
		bestResp *llm.ChatResponse
		bestSim  float32 = -2 // 余弦相似度最小为 -1，初始值设低于此值
	)

	for _, e := range s.entries {
		if !e.expireAt.After(now) {
			// 跳过已过期条目.
			continue
		}
		sim := cosineSimilarity(query, e.vector)
		if sim >= threshold && sim > bestSim {
			bestSim = sim
			bestResp = e.response
		}
	}
	return bestResp, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// Clear 清空内存中所有缓存条目.
func (s *MemoryStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	return nil
}

// 编译期接口断言.
var _ Store = (*MemoryStore)(nil)

var (
	// ErrNilConfig 表示缓存配置为空.
	ErrNilConfig = errors.New("cache: config is nil")
	// ErrNilEmbeddingModel 表示未配置嵌入模型.
	ErrNilEmbeddingModel = errors.New("cache: embedding model is nil")
	// ErrNilStore 表示未配置缓存存储.
	ErrNilStore = errors.New("cache: store is nil")
)

// extractLastUserText 从消息列表中提取最后一条用户消息的文本内容.
// 同时兼容 Content 字段和 Parts 中的文本片段.
// 若没有用户消息则返回空字符串.
func extractLastUserText(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != llm.RoleUser {
			continue
		}
		if msg.Content != "" {
			return msg.Content
		}
		// 尝试从多模态 Parts 中拼接文本.
		var text string
		for _, p := range msg.Parts {
			if p.Type == llm.ContentTypeText {
				text += p.Text
			}
		}
		if text != "" {
			return text
		}
	}
	return ""
}

// cacheHitReader 将缓存响应包装为 StreamReader，直接以 EOF 结束.
type cacheHitReader struct {
	resp *llm.ChatResponse
	done bool
}

// Recv 首次调用返回完整 delta，之后返回 io.EOF.
func (r *cacheHitReader) Recv() (llm.StreamChunk, error) {
	if r.done {
		return llm.StreamChunk{}, io.EOF
	}
	r.done = true
	return llm.StreamChunk{
		Delta:        r.resp.Message.Content,
		FinishReason: r.resp.FinishReason,
	}, nil
}

// Response 返回缓存的完整响应.
func (r *cacheHitReader) Response() *llm.ChatResponse { return r.resp }

// Close 无需释放资源，直接返回 nil.
func (r *cacheHitReader) Close() error { return nil }

// Middleware 返回语义缓存中间件，对 Generate 和 Stream 均生效.
// 命中缓存时直接返回已缓存响应，未命中时调用下游模型并将结果缓存.
func Middleware(cfg *Config) (middleware.Middleware, error) {
	cfg, err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}

	return func(next llm.ChatModel) llm.ChatModel {
		return middleware.Wrap(
			// Generate：先查缓存，未命中则调用模型并缓存结果.
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				text := extractLastUserText(messages)
				if text == "" {
					return next.Generate(ctx, messages, opts...)
				}

				// 嵌入查询文本.
				queryVec, ok := embedQuery(ctx, cfg.EmbeddingModel, text)
				if !ok {
					return next.Generate(ctx, messages, opts...)
				}
				// 查找缓存.
				cached, err := cfg.Store.Search(ctx, queryVec, cfg.Threshold)
				if err == nil && cached != nil {
					// 缓存命中，直接返回.
					return cached, nil
				}

				// 调用下游模型.
				resp, err := next.Generate(ctx, messages, opts...)
				if err != nil {
					return nil, err
				}

				// 将结果存入缓存（嵌入向量已经生成，直接使用）.
				_ = cfg.Store.Put(ctx, queryVec, resp, cfg.TTL)
				return resp, nil
			},
			// Stream：先查缓存，命中时返回包装好的 StreamReader，未命中则透传.
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				text := extractLastUserText(messages)
				if text == "" {
					return next.Stream(ctx, messages, opts...)
				}

				// 嵌入查询文本.
				queryVec, ok := embedQuery(ctx, cfg.EmbeddingModel, text)
				if !ok {
					return next.Stream(ctx, messages, opts...)
				}

				// 查找缓存.
				cached, err := cfg.Store.Search(ctx, queryVec, cfg.Threshold)
				if err == nil && cached != nil {
					return &cacheHitReader{resp: cached}, nil
				}

				// 调用下游模型.
				reader, err := next.Stream(ctx, messages, opts...)
				if err != nil {
					return nil, err
				}

				// 流式场景：用包装器在读完所有 chunk 后将完整响应存入缓存.
				return &cachingStreamReader{
					inner:    reader,
					store:    cfg.Store,
					queryVec: queryVec,
					ttl:      cfg.TTL,
					ctx:      ctx,
				}, nil
			},
		)
	}, nil
}

func validateConfig(cfg *Config) (*Config, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	cfg = cfg.withDefaults()
	if cfg.EmbeddingModel == nil {
		return nil, ErrNilEmbeddingModel
	}
	if cfg.Store == nil {
		return nil, ErrNilStore
	}
	return cfg, nil
}

func embedQuery(ctx context.Context, model llm.EmbeddingModel, text string) ([]float32, bool) {
	embedResp, err := model.EmbedTexts(ctx, []string{text})
	if err != nil || embedResp == nil || len(embedResp.Embeddings) == 0 || len(embedResp.Embeddings[0]) == 0 {
		return nil, false
	}
	return embedResp.Embeddings[0], true
}

// cachingStreamReader 包装 StreamReader，在流读取完毕后自动将响应存入缓存.
type cachingStreamReader struct {
	inner    llm.StreamReader
	store    Store
	queryVec []float32
	ttl      time.Duration
	ctx      context.Context //nolint:containedctx
	stored   bool
}

// Recv 透传内部 StreamReader 的 Recv，遇到 EOF 时触发缓存写入.
func (r *cachingStreamReader) Recv() (llm.StreamChunk, error) {
	chunk, err := r.inner.Recv()
	if errors.Is(err, io.EOF) && !r.stored {
		r.stored = true
		if resp := r.inner.Response(); resp != nil {
			_ = r.store.Put(r.ctx, r.queryVec, resp, r.ttl)
		}
	}
	return chunk, err
}

// Response 返回内部读取器的完整响应.
func (r *cachingStreamReader) Response() *llm.ChatResponse { return r.inner.Response() }

// Close 关闭内部读取器.
func (r *cachingStreamReader) Close() error { return r.inner.Close() }

// NewCachedModel 将语义缓存中间件应用到 model，返回带缓存能力的 ChatModel.
func NewCachedModel(model llm.ChatModel, cfg *Config) (llm.ChatModel, error) {
	mw, err := Middleware(cfg)
	if err != nil {
		return nil, err
	}
	return mw(model), nil
}
