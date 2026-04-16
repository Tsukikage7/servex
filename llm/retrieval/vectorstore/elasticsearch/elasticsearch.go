// Package elasticsearch 提供基于 Elasticsearch kNN 的向量存储实现，支持混合搜索.
package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
)

// Config Elasticsearch 向量存储配置.
type Config struct {
	// Client ES 客户端，必填.
	Client *es.Client
	// IndexName 索引名称，必填.
	IndexName string
	// Dimension 向量维度，必填.
	Dimension int
}

// Store Elasticsearch 向量存储.
type Store struct {
	cfg Config
}

// New 创建 Elasticsearch Store 实例.
func New(_ context.Context, cfg Config) (*Store, error) {
	if cfg.Client == nil {
		return nil, errors.New("elasticsearch: client is nil")
	}
	if cfg.IndexName == "" {
		return nil, errors.New("elasticsearch: index name is empty")
	}
	if cfg.Dimension <= 0 {
		return nil, errors.New("elasticsearch: dimension must be > 0")
	}
	return &Store{cfg: cfg}, nil
}

// AutoMigrate 幂等创建索引及 mappings.
func (s *Store) AutoMigrate(ctx context.Context) error {
	body := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"content": map[string]any{
					"type": "text",
				},
				"vector": map[string]any{
					"type":       "dense_vector",
					"dims":       s.cfg.Dimension,
					"index":      true,
					"similarity": "cosine",
				},
				"metadata": map[string]any{
					"type":    "object",
					"enabled": true,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("elasticsearch: marshal mappings: %w", err)
	}

	res, err := s.cfg.Client.Indices.Create(
		s.cfg.IndexName,
		s.cfg.Client.Indices.Create.WithContext(ctx),
		s.cfg.Client.Indices.Create.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// 索引已存在时忽略
		var errResp map[string]any
		if jsonErr := json.NewDecoder(res.Body).Decode(&errResp); jsonErr == nil {
			if errType, ok := extractErrorType(errResp); ok && errType == "resource_already_exists_exception" {
				return nil
			}
		}
		return fmt.Errorf("elasticsearch: create index response %s", res.Status())
	}
	return nil
}

// AddDocuments 使用 Bulk API 批量索引文档.
func (s *Store) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, doc := range docs {
		// action line
		meta := map[string]any{
			"index": map[string]any{
				"_index": s.cfg.IndexName,
				"_id":    doc.ID,
			},
		}
		metaBytes, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("elasticsearch: marshal bulk action for %q: %w", doc.ID, err)
		}
		buf.Write(metaBytes)
		buf.WriteByte('\n')

		// document body
		docBody := map[string]any{
			"content":  doc.Content,
			"vector":   doc.Vector,
			"metadata": doc.Metadata,
		}
		docBytes, err := json.Marshal(docBody)
		if err != nil {
			return fmt.Errorf("elasticsearch: marshal doc %q: %w", doc.ID, err)
		}
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	res, err := s.cfg.Client.Bulk(
		bytes.NewReader(buf.Bytes()),
		s.cfg.Client.Bulk.WithContext(ctx),
		s.cfg.Client.Bulk.WithIndex(s.cfg.IndexName),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: bulk index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch: bulk index response %s", res.Status())
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Error *struct {
				Reason string `json:"reason"`
			} `json:"error,omitempty"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&bulkResp); err != nil {
		return fmt.Errorf("elasticsearch: decode bulk response: %w", err)
	}
	if bulkResp.Errors {
		for _, item := range bulkResp.Items {
			for _, v := range item {
				if v.Error != nil {
					return fmt.Errorf("elasticsearch: bulk index error: %s", v.Error.Reason)
				}
			}
		}
	}
	return nil
}

// SimilaritySearch 基于 kNN（可选混合搜索）查询最相似的 k 条文档.
func (s *Store) SimilaritySearch(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
	o := vectorstore.ApplySearchOptions(opts)

	knn := map[string]any{
		"field":          "vector",
		"query_vector":   query,
		"k":              k,
		"num_candidates": k * 2,
	}

	if o.Filter() != nil {
		var filterTerms []map[string]any
		for key, val := range o.Filter() {
			filterTerms = append(filterTerms, map[string]any{
				"term": map[string]any{
					"metadata." + key: val,
				},
			})
		}
		if len(filterTerms) == 1 {
			knn["filter"] = filterTerms[0]
		} else {
			knn["filter"] = map[string]any{"bool": map[string]any{"must": filterTerms}}
		}
	}

	searchBody := map[string]any{
		"knn":     knn,
		"size":    k,
		"_source": []string{"content", "metadata"},
	}

	if tq := o.TextQuery(); tq != "" {
		searchBody["query"] = map[string]any{
			"match": map[string]any{
				"content": tq,
			},
		}
	}

	bodyBytes, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: marshal search body: %w", err)
	}

	res, err := s.cfg.Client.Search(
		s.cfg.Client.Search.WithContext(ctx),
		s.cfg.Client.Search.WithIndex(s.cfg.IndexName),
		s.cfg.Client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch: search response %s", res.Status())
	}

	var searchResp struct {
		Hits struct {
			Hits []struct {
				ID     string  `json:"_id"`
				Score  float32 `json:"_score"`
				Source struct {
					Content  string         `json:"content"`
					Metadata map[string]any `json:"metadata"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("elasticsearch: decode search response: %w", err)
	}

	var results []vectorstore.SearchResult
	for _, hit := range searchResp.Hits.Hits {
		if o.ScoreThreshold() != nil && hit.Score < *o.ScoreThreshold() {
			continue
		}
		results = append(results, vectorstore.SearchResult{
			Document: vectorstore.Document{
				ID:       hit.ID,
				Content:  hit.Source.Content,
				Metadata: hit.Source.Metadata,
			},
			Score: hit.Score,
		})
	}
	return results, nil
}

// Delete 使用 Bulk API 批量删除文档.
func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, id := range ids {
		action := map[string]any{
			"delete": map[string]any{
				"_index": s.cfg.IndexName,
				"_id":    id,
			},
		}
		actionBytes, err := json.Marshal(action)
		if err != nil {
			return fmt.Errorf("elasticsearch: marshal delete action for %q: %w", id, err)
		}
		buf.Write(actionBytes)
		buf.WriteByte('\n')
	}

	res, err := s.cfg.Client.Bulk(
		bytes.NewReader(buf.Bytes()),
		s.cfg.Client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: bulk delete: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch: bulk delete response %s", res.Status())
	}
	return nil
}

// extractErrorType 从 ES 错误响应中提取错误类型.
func extractErrorType(resp map[string]any) (string, bool) {
	errField, ok := resp["error"]
	if !ok {
		return "", false
	}
	switch v := errField.(type) {
	case map[string]any:
		if t, ok := v["type"].(string); ok {
			return t, true
		}
	case string:
		if strings.Contains(v, "resource_already_exists_exception") {
			return "resource_already_exists_exception", true
		}
	}
	return "", false
}

// 编译期接口断言.
var _ vectorstore.VectorStore = (*Store)(nil)
