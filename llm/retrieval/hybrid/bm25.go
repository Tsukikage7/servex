package hybrid

import (
	"cmp"
	"context"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// BM25 默认参数.
const (
	// defaultBM25K1 词频饱和参数，典型值 1.2~2.0.
	defaultBM25K1 = 1.5
	// defaultBM25B 长度归一化参数，典型值 0.75.
	defaultBM25B = 0.75
)

// BM25Retriever 内存 BM25 词法召回器.
//
// 分词策略：ASCII 字母/数字累积成 token 并小写化；CJK 每字独立成 token；其他字符作分隔符.
// 适合中小规模语料（< 10 万文档）；索引在 NewBM25Retriever 时一次性构建.
// 索引构建后不应修改 docs 或调用 NewBM25Retriever 时的原 slice — 本实现已做浅拷贝，但文档内 Metadata 仍共享.
type BM25Retriever struct {
	// docs 原始文档列表（按索引保留顺序）.
	docs []rag.Document
	// tokenized 每篇文档的 token 序列.
	tokenized [][]string
	// tfs 每篇文档的词频表：term → count，查询期直接读取避免重复扫描 tokens.
	tfs []map[string]int
	// df 文档频率：token → 出现过该 token 的文档数.
	df map[string]int
	// avgdl 语料平均文档长度（以 token 计）.
	avgdl float64
	// k1 词频饱和参数.
	k1 float64
	// b 长度归一化参数.
	b float64
}

// BM25Option BM25 召回器配置选项.
type BM25Option func(*BM25Retriever)

// WithBM25Params 设置 BM25 的 k1 与 b 参数.
// k1 控制词频饱和速率（典型 1.2~2.0），b 控制长度归一化强度（0~1，1 表示完全按平均长度归一化）.
func WithBM25Params(k1, b float64) BM25Option {
	return func(r *BM25Retriever) {
		r.k1 = k1
		r.b = b
	}
}

// NewBM25Retriever 创建 BM25 召回器并一次性建立内存索引.
// docs 为语料库；默认 k1=1.5，b=0.75.
// 内部对 docs 做浅拷贝，调用方后续修改原 slice 不会破坏已构建的索引.
func NewBM25Retriever(docs []rag.Document, opts ...BM25Option) *BM25Retriever {
	r := &BM25Retriever{
		docs: slices.Clone(docs),
		k1:   defaultBM25K1,
		b:    defaultBM25B,
	}
	for _, opt := range opts {
		opt(r)
	}
	r.buildIndex()
	return r
}

// buildIndex 为当前 docs 构建倒排所需的统计信息：tokenized、tfs、df、avgdl.
func (r *BM25Retriever) buildIndex() {
	r.tokenized = make([][]string, len(r.docs))
	r.tfs = make([]map[string]int, len(r.docs))
	r.df = make(map[string]int)

	var totalLen int
	for i, d := range r.docs {
		tokens := tokenize(d.Content)
		r.tokenized[i] = tokens
		totalLen += len(tokens)

		// 构建 per-doc 词频表，查询时直接读.
		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}
		r.tfs[i] = tf

		// 统计 df：同一文档内同一 token 只计一次，直接复用 tf 的 key.
		for tok := range tf {
			r.df[tok]++
		}
	}
	if len(r.docs) > 0 {
		r.avgdl = float64(totalLen) / float64(len(r.docs))
	}
}

// Retrieve 按 BM25 打分并返回 topK 条相关文档，score > 0 才入选.
// 空语料或空查询返回空结果，不视为错误.
// 空语料或空 query 返回 (nil, nil)；混合检索场景中，一路空不应当作错误.
func (r *BM25Retriever) Retrieve(ctx context.Context, query string, topK int) ([]rag.RetrievedDoc, error) {
	_, span := startBM25RetrieveSpan(ctx, query, topK, len(r.docs))
	defer span.End()

	if len(r.docs) == 0 {
		recordBM25Result(span, 0)
		return nil, nil
	}
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		recordBM25Result(span, 0)
		return nil, nil
	}

	// 去重 query token 防止重复累加 IDF.
	qSet := make(map[string]struct{}, len(qTokens))
	uniqueQ := make([]string, 0, len(qTokens))
	for _, tok := range qTokens {
		if _, ok := qSet[tok]; ok {
			continue
		}
		qSet[tok] = struct{}{}
		uniqueQ = append(uniqueQ, tok)
	}

	n := float64(len(r.docs))
	results := make([]rag.RetrievedDoc, 0, len(r.docs))

	for i, d := range r.docs {
		score := r.scoreDoc(i, uniqueQ, n)
		if score <= 0 {
			continue
		}
		results = append(results, rag.RetrievedDoc{
			Document: rag.Document{
				ID:       d.ID,
				Content:  d.Content,
				Metadata: d.Metadata,
			},
			Score: float32(score),
		})
	}

	// 按分数降序排序.
	slices.SortStableFunc(results, func(a, b rag.RetrievedDoc) int {
		return cmp.Compare(b.Score, a.Score)
	})
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	recordBM25Result(span, len(results))
	return results, nil
}

// scoreDoc 计算单篇文档相对 query tokens 的 BM25 分数.
//
// score(q, d) = Σ IDF(qi) * f(qi,d)*(k1+1) / (f(qi,d) + k1*(1 - b + b*|d|/avgdl))
// IDF(qi)    = ln((N - n(qi) + 0.5) / (n(qi) + 0.5) + 1)
func (r *BM25Retriever) scoreDoc(docIdx int, qTokens []string, n float64) float64 {
	tokens := r.tokenized[docIdx]
	if len(tokens) == 0 {
		return 0
	}
	// 直接读取预构建的 per-doc 词频表，避免每次查询重建 map.
	tf := r.tfs[docIdx]

	dl := float64(len(tokens))
	avgdl := r.avgdl
	if avgdl == 0 {
		avgdl = dl
	}

	var score float64
	for _, q := range qTokens {
		f := tf[q]
		if f == 0 {
			continue
		}
		df := float64(r.df[q])
		idf := math.Log((n-df+0.5)/(df+0.5) + 1)
		numer := float64(f) * (r.k1 + 1)
		denom := float64(f) + r.k1*(1-r.b+r.b*dl/avgdl)
		score += idf * numer / denom
	}
	return score
}

// tokenize 将文本拆分为 token.
// 规则：
//   - ASCII 字母/数字累积成一个 token，整体小写化；
//   - CJK（含 Han 区段）每个 rune 作为独立 token；
//   - 其他字符（空白、标点、下划线等）作为分隔符.
func tokenize(text string) []string {
	if text == "" {
		return nil
	}
	var (
		tokens []string
		buf    strings.Builder
	)
	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, strings.ToLower(buf.String()))
			buf.Reset()
		}
	}
	for _, r := range text {
		switch {
		case r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			buf.WriteRune(r)
		case unicode.Is(unicode.Han, r):
			flush()
			tokens = append(tokens, string(r))
		default:
			flush()
		}
	}
	flush()
	return tokens
}
