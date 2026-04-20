// Package hybrid 提供混合检索：向量召回 + 词法召回（BM25）通过 RRF（Reciprocal Rank Fusion）融合.
//
// 对两路召回结果按排名位置加权求和，可在同一查询下兼顾语义相似度与关键词命中，
// 常用于客服问答、文档搜索等 RAG 场景的召回阶段.
package hybrid

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// 默认配置.
const (
	// defaultRRFK RRF 算法中的常数 k，文献经验值 60.
	defaultRRFK = 60
	// defaultWeight 两路召回的默认权重.
	defaultWeight = 1.0
)

// Retriever 抽象召回器接口：给定 query 与 topK，返回按相关度降序排列的文档.
type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]rag.RetrievedDoc, error)
}

// HybridRetriever 混合召回器，并发调用向量与词法两路并做 RRF 融合.
type HybridRetriever struct {
	// vector 向量召回器.
	vector Retriever
	// lexical 词法召回器.
	lexical Retriever
	// k RRF 公式中的常数.
	k int
	// vWeight 向量路权重.
	vWeight float32
	// lWeight 词法路权重.
	lWeight float32
}

// Option 混合召回器配置选项.
type Option func(*HybridRetriever)

// WithRRFK 设置 RRF 公式中的常数 k，默认 60.
func WithRRFK(k int) Option {
	return func(h *HybridRetriever) {
		h.k = k
	}
}

// WithWeights 设置向量路与词法路的权重，默认均为 1.0.
func WithWeights(vec, lex float32) Option {
	return func(h *HybridRetriever) {
		h.vWeight = vec
		h.lWeight = lex
	}
}

// New 创建 HybridRetriever.
// 默认 k=60，向量权重=1.0，词法权重=1.0.
func New(vector, lexical Retriever, opts ...Option) *HybridRetriever {
	h := &HybridRetriever{
		vector:  vector,
		lexical: lexical,
		k:       defaultRRFK,
		vWeight: defaultWeight,
		lWeight: defaultWeight,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// retrieveResult 单路召回的结果包装.
type retrieveResult struct {
	// docs 召回文档.
	docs []rag.RetrievedDoc
	// err 召回错误.
	err error
}

// Retrieve 并发两路召回后 RRF 融合.
// 两路均取 topK*2 作为候选池以提升融合质量；最终按融合分数降序截取 topK.
// 语义：一路失败另一路仍返回结果（err 为 nil）；两路都失败才返回 joined error.
//
// 行为约定:
//   - 两路并发执行，ctx 的取消/超时传播依赖底层 Retriever 实现
//   - 允许 vector 或 lexical 为 nil，该路视为空结果（RRF 仍在另一路上工作）
//   - topK <= 0 时返回全部融合结果
//   - 两路都报错时返回 errors.Join(vErr, lErr)
func (h *HybridRetriever) Retrieve(ctx context.Context, query string, topK int) ([]rag.RetrievedDoc, error) {
	ctx, span := startHybridRetrieveSpan(ctx, query, topK, h.k, h.vWeight, h.lWeight)
	defer span.End()

	// 候选池扩大以提升融合质量.
	pool := topK * 2
	if pool <= 0 {
		pool = topK
	}

	vc := make(chan retrieveResult, 1)
	lc := make(chan retrieveResult, 1)

	go func() {
		var rr retrieveResult
		if h.vector != nil {
			rr.docs, rr.err = h.vector.Retrieve(ctx, query, pool)
		}
		vc <- rr
	}()
	go func() {
		var rr retrieveResult
		if h.lexical != nil {
			rr.docs, rr.err = h.lexical.Retrieve(ctx, query, pool)
		}
		lc <- rr
	}()

	vr := <-vc
	lr := <-lc

	fused := rrfFuse(vr.docs, lr.docs, h.k, h.vWeight, h.lWeight)
	if topK > 0 && len(fused) > topK {
		fused = fused[:topK]
	}

	// 两路都失败才返回错误，单路失败仍返回另一路结果.
	if vr.err != nil && lr.err != nil {
		err := errors.Join(vr.err, lr.err)
		recordSpanError(span, err)
		return fused, err
	}
	recordHybridResult(span, len(fused))
	return fused, nil
}

// rrfFuse 按 RRF 公式对两路召回结果加权融合.
//
// 公式：score(doc) = vw * 1/(k + vRank) + lw * 1/(k + lRank)
// 同一 ID 在两路中出现则贡献累加；rank 从 0 开始计数.
// 累加容器使用 float64 以降低浮点误差，返回时再转为 float32 存入 Score.
// 返回按分数降序排列的融合结果（保留所有出现过的文档）.
func rrfFuse(vec, lex []rag.RetrievedDoc, k int, vw, lw float32) []rag.RetrievedDoc {
	scores := make(map[string]float64, len(vec)+len(lex))
	docs := make(map[string]rag.Document, len(vec)+len(lex))

	fuseOne := func(list []rag.RetrievedDoc, w float32) {
		for rank, d := range list {
			scores[d.ID] += float64(w) * (1.0 / float64(k+rank+1))
			if _, ok := docs[d.ID]; !ok {
				docs[d.ID] = d.Document
			}
		}
	}
	fuseOne(vec, vw)
	fuseOne(lex, lw)

	out := make([]rag.RetrievedDoc, 0, len(scores))
	for id, s := range scores {
		out = append(out, rag.RetrievedDoc{Document: docs[id], Score: float32(s)})
	}
	slices.SortStableFunc(out, func(a, b rag.RetrievedDoc) int {
		return cmp.Compare(b.Score, a.Score)
	})
	return out
}
