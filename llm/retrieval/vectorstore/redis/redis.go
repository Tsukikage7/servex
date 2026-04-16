// Package redis 提供基于 Redis Stack（RediSearch）的向量存储实现.
package redis

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	goredis "github.com/redis/go-redis/v9"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
)

// Config Redis Search 配置.
type Config struct {
	Client    *goredis.Client
	IndexName string
	Dimension int
}

// Store Redis Search 向量存储.
type Store struct {
	cfg    Config
	prefix string
}

// New 创建 Redis Store 实例.
func New(_ context.Context, cfg Config) (*Store, error) {
	if cfg.Client == nil {
		return nil, errors.New("redis: client is nil")
	}
	if cfg.IndexName == "" {
		return nil, errors.New("redis: index name is empty")
	}
	if cfg.Dimension <= 0 {
		return nil, errors.New("redis: dimension must be > 0")
	}
	return &Store{cfg: cfg, prefix: cfg.IndexName + ":"}, nil
}

// AutoMigrate 幂等创建索引（已存在时忽略）.
func (s *Store) AutoMigrate(ctx context.Context) error {
	cmd := s.cfg.Client.Do(ctx,
		"FT.CREATE", s.cfg.IndexName,
		"ON", "HASH",
		"PREFIX", "1", s.prefix,
		"SCHEMA",
		"content", "TEXT",
		"metadata", "TEXT",
		"vector", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", fmt.Sprintf("%d", s.cfg.Dimension),
		"DISTANCE_METRIC", "COSINE",
	)
	if err := cmd.Err(); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Index already exists") {
			return nil
		}
		return fmt.Errorf("redis: FT.CREATE: %w", err)
	}
	return nil
}

// AddDocuments 批量写入文档（Hash 结构）.
func (s *Store) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	for _, doc := range docs {
		key := s.prefix + doc.ID
		meta, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("redis: marshal metadata for %q: %w", doc.ID, err)
		}
		vecBytes := float32SliceToBytes(doc.Vector)
		if err := s.cfg.Client.HSet(ctx, key,
			"id", doc.ID,
			"content", doc.Content,
			"metadata", string(meta),
			"vector", vecBytes,
		).Err(); err != nil {
			return fmt.Errorf("redis: hset %q: %w", key, err)
		}
	}
	return nil
}

// SimilaritySearch 基于 COSINE KNN 搜索.
//
// go-redis/v9 使用 RESP3 协议，FT.SEARCH 返回 map[any]any 格式：
//
//	{
//	  "total_results": N,
//	  "results": [{"id": "key", "extra_attributes": {"id":…, "content":…, "score":…}}, …],
//	  …
//	}
func (s *Store) SimilaritySearch(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
	o := vectorstore.ApplySearchOptions(opts)
	queryBytes := float32SliceToBytes(query)

	knnQuery := fmt.Sprintf("*=>[KNN %d @vector $vec AS score]", k)
	cmd := s.cfg.Client.Do(ctx,
		"FT.SEARCH", s.cfg.IndexName, knnQuery,
		"PARAMS", "2", "vec", queryBytes,
		"SORTBY", "score",
		"RETURN", "4", "id", "content", "metadata", "score",
		"DIALECT", "2",
	)
	if err := cmd.Err(); err != nil {
		return nil, fmt.Errorf("redis: FT.SEARCH: %w", err)
	}

	// go-redis/v9 negotiates RESP3; FT.SEARCH returns a map.
	respMap, ok := cmd.Val().(map[any]any)
	if !ok {
		// 兜底：可能是 RESP2 返回的 []any，或空结果
		if cmd.Val() == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis: unexpected FT.SEARCH response type: %T", cmd.Val())
	}
	rows, _ := respMap["results"].([]any)

	var results []vectorstore.SearchResult
	for _, row := range rows {
		rowMap, ok := row.(map[any]any)
		if !ok {
			continue
		}
		attrs, _ := rowMap["extra_attributes"].(map[any]any)

		strAttr := func(key string) string {
			v, _ := attrs[key].(string)
			return v
		}

		var dist float64
		fmt.Sscanf(strAttr("score"), "%f", &dist)
		score := float32(1 - dist)
		if score < 0 {
			score = 0
		}

		if o.ScoreThreshold() != nil && score < *o.ScoreThreshold() {
			continue
		}

		var meta map[string]any
		if m := strAttr("metadata"); m != "" && m != "null" {
			_ = json.Unmarshal([]byte(m), &meta)
		}

		if o.Filter() != nil && !matchFilter(meta, o.Filter()) {
			continue
		}

		results = append(results, vectorstore.SearchResult{
			Document: vectorstore.Document{
				ID:       strAttr("id"),
				Content:  strAttr("content"),
				Metadata: meta,
			},
			Score: score,
		})
	}
	return results, nil
}

// Delete 根据 ID 列表删除 Hash key.
func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = s.prefix + id
	}
	return s.cfg.Client.Del(ctx, keys...).Err()
}

// float32SliceToBytes 将 []float32 转为小端字节序.
func float32SliceToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func matchFilter(meta, filter map[string]any) bool {
	if meta == nil {
		return false
	}
	for k, v := range filter {
		if meta[k] != v {
			return false
		}
	}
	return true
}

// 编译期接口断言.
var _ vectorstore.VectorStore = (*Store)(nil)
