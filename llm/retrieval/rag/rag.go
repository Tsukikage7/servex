// Package rag 提供检索增强生成（RAG）管线实现.
//
// 支持文档导入（分块、嵌入、存储）、语义检索以及检索增强生成（非流式/流式）.
package rag

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/prompt"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/embedding"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/splitter"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
)

// 预定义错误.
var (
	// ErrNilChatModel 未设置聊天模型.
	ErrNilChatModel = errors.New("rag: chat model is nil")
	// ErrNilEmbeddingModel 未设置嵌入模型.
	ErrNilEmbeddingModel = errors.New("rag: embedding model is nil")
	// ErrNilVectorStore 未设置向量存储.
	ErrNilVectorStore = errors.New("rag: vector store is nil")
	// ErrNoResults 未检索到相关文档.
	ErrNoResults = errors.New("rag: no relevant documents found")
)

// defaultPromptText 默认系统提示词模板文本.
const defaultPromptText = `基于以下参考资料回答用户问题。如果参考资料中没有相关信息，请说明你无法回答。

参考资料：
{{range .Sources}}
---
{{.Content}}
---
{{end}}

用户问题：{{.Question}}`

// Document 待导入文档.
type Document struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitzero"`
}

// RetrievedDoc 检索结果.
type RetrievedDoc struct {
	Document
	// Score 相似度分数.
	Score float32 `json:"score"`
}

// Result RAG 结果.
type Result struct {
	// Answer 模型生成的回答.
	Answer string `json:"answer"`
	// Sources 用于生成回答的检索文档列表.
	Sources []RetrievedDoc `json:"sources"`
	// Usage token 用量统计.
	Usage llm.Usage `json:"usage"`
}

// Config RAG 管线配置.
type Config struct {
	// ChatModel 聊天模型（必填）.
	ChatModel llm.ChatModel
	// EmbeddingModel 嵌入模型（必填）.
	EmbeddingModel llm.EmbeddingModel
	// VectorStore 向量存储（必填）.
	VectorStore vectorstore.VectorStore
	// Splitter 文本分块器（可选，用于 Ingest 分块）.
	Splitter splitter.Splitter
	// TopK 检索数量，默认 5.
	TopK int
	// ScoreThreshold 最低相关度，默认 0.
	ScoreThreshold float32
	// PromptTemplate 自定义提示词模板（可选）.
	PromptTemplate *prompt.Template
}

// Pipeline RAG 管线.
type Pipeline struct {
	cfg *Config
}

// New 创建 RAG 管线，验证必填配置并应用默认值.
func New(cfg *Config) (*Pipeline, error) {
	if cfg.ChatModel == nil {
		return nil, ErrNilChatModel
	}
	if cfg.EmbeddingModel == nil {
		return nil, ErrNilEmbeddingModel
	}
	if cfg.VectorStore == nil {
		return nil, ErrNilVectorStore
	}

	// 应用默认值.
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}

	// 未提供模板时使用默认提示词模板.
	if cfg.PromptTemplate == nil {
		tmpl, err := prompt.New(llm.RoleSystem, defaultPromptText)
		if err != nil {
			return nil, fmt.Errorf("rag: 创建默认提示词模板失败: %w", err)
		}
		cfg.PromptTemplate = tmpl
	}

	return &Pipeline{cfg: cfg}, nil
}

// Ingest 导入文档：分块（可选）→ 嵌入 → 存入向量库.
func (p *Pipeline) Ingest(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	// 收集所有待嵌入的文本及对应的向量库文档.
	var texts []string
	var vsDocs []vectorstore.Document

	for _, doc := range docs {
		if p.cfg.Splitter != nil {
			// 使用分块器将文档拆分为多个块.
			chunks := p.cfg.Splitter.Split(doc.Content)
			for i, chunk := range chunks {
				// 合并文档元数据与块元数据.
				meta := make(map[string]any)
				for k, v := range doc.Metadata {
					meta[k] = v
				}
				for k, v := range chunk.Metadata {
					meta[k] = v
				}
				meta["source_doc_id"] = doc.ID
				meta["chunk_index"] = chunk.Index

				texts = append(texts, chunk.Text)
				vsDocs = append(vsDocs, vectorstore.Document{
					ID:       fmt.Sprintf("%s_%d", doc.ID, i),
					Content:  chunk.Text,
					Metadata: meta,
				})
			}
		} else {
			// 不分块，整篇文档作为一个单元.
			meta := make(map[string]any)
			for k, v := range doc.Metadata {
				meta[k] = v
			}
			texts = append(texts, doc.Content)
			vsDocs = append(vsDocs, vectorstore.Document{
				ID:       doc.ID,
				Content:  doc.Content,
				Metadata: meta,
			})
		}
	}

	if len(texts) == 0 {
		return nil
	}

	// 批量嵌入，每批最多 100 条.
	embedResp, err := embedding.BatchEmbed(ctx, p.cfg.EmbeddingModel, texts, 100)
	if err != nil {
		return fmt.Errorf("rag: 嵌入文档失败: %w", err)
	}

	// 将嵌入向量写入对应的向量库文档.
	for i := range vsDocs {
		if i < len(embedResp.Embeddings) {
			vsDocs[i].Vector = embedResp.Embeddings[i]
		}
	}

	// 存入向量库.
	if err := p.cfg.VectorStore.AddDocuments(ctx, vsDocs); err != nil {
		return fmt.Errorf("rag: 存储文档失败: %w", err)
	}

	return nil
}

// Retrieve 只检索不生成，返回与问题最相关的文档列表.
func (p *Pipeline) Retrieve(ctx context.Context, question string) ([]RetrievedDoc, error) {
	// 嵌入问题文本.
	embedResp, err := p.cfg.EmbeddingModel.EmbedTexts(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("rag: 嵌入问题失败: %w", err)
	}
	if len(embedResp.Embeddings) == 0 {
		return nil, fmt.Errorf("rag: 嵌入模型未返回向量")
	}

	queryVec := embedResp.Embeddings[0]

	// 构建搜索选项.
	var searchOpts []vectorstore.SearchOption
	if p.cfg.ScoreThreshold > 0 {
		searchOpts = append(searchOpts, vectorstore.WithScoreThreshold(p.cfg.ScoreThreshold))
	}

	// 在向量库中搜索.
	results, err := p.cfg.VectorStore.SimilaritySearch(ctx, queryVec, p.cfg.TopK, searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("rag: 向量搜索失败: %w", err)
	}

	// 转换为 RetrievedDoc.
	retrieved := make([]RetrievedDoc, 0, len(results))
	for _, r := range results {
		retrieved = append(retrieved, RetrievedDoc{
			Document: Document{
				ID:       r.Document.ID,
				Content:  r.Document.Content,
				Metadata: r.Document.Metadata,
			},
			Score: r.Score,
		})
	}

	return retrieved, nil
}

// promptData 提示词模板渲染数据.
type promptData struct {
	// Sources 检索到的参考文档.
	Sources []RetrievedDoc
	// Question 用户问题.
	Question string
}

// Query 检索增强生成：检索 → 渲染提示词 → 调用聊天模型生成回答.
func (p *Pipeline) Query(ctx context.Context, question string, opts ...llm.CallOption) (*Result, error) {
	// 检索相关文档.
	sources, err := p.Retrieve(ctx, question)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, ErrNoResults
	}

	// 渲染提示词模板.
	sysMsg, err := p.cfg.PromptTemplate.Render(promptData{
		Sources:  sources,
		Question: question,
	})
	if err != nil {
		return nil, fmt.Errorf("rag: 渲染提示词失败: %w", err)
	}

	// 调用聊天模型生成回答.
	messages := []llm.Message{
		sysMsg,
		llm.UserMessage(question),
	}
	resp, err := p.cfg.ChatModel.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("rag: 生成回答失败: %w", err)
	}

	return &Result{
		Answer:  resp.Message.Content,
		Sources: sources,
		Usage:   resp.Usage,
	}, nil
}

// QueryStream 流式检索增强生成：检索 → 渲染提示词 → 流式调用聊天模型.
func (p *Pipeline) QueryStream(ctx context.Context, question string, opts ...llm.CallOption) (llm.StreamReader, error) {
	// 检索相关文档.
	sources, err := p.Retrieve(ctx, question)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, ErrNoResults
	}

	// 渲染提示词模板.
	sysMsg, err := p.cfg.PromptTemplate.Render(promptData{
		Sources:  sources,
		Question: question,
	})
	if err != nil {
		return nil, fmt.Errorf("rag: 渲染提示词失败: %w", err)
	}

	// 流式调用聊天模型.
	messages := []llm.Message{
		sysMsg,
		llm.UserMessage(question),
	}
	reader, err := p.cfg.ChatModel.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("rag: 流式生成失败: %w", err)
	}

	return reader, nil
}
