// Package pgvector 提供基于 PostgreSQL pgvector 扩展的向量存储实现.
package pgvector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
)

var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Config pgvector 配置.
type Config struct {
	Pool      *pgxpool.Pool
	Table     string
	Dimension int
}

// Store pgvector 向量存储.
type Store struct {
	cfg Config
}

// New 创建 pgvector Store 实例.
func New(_ context.Context, cfg Config) (*Store, error) {
	if cfg.Pool == nil {
		return nil, errors.New("pgvector: pool is nil")
	}
	if cfg.Table == "" {
		return nil, errors.New("pgvector: table name is empty")
	}
	if !validTableName.MatchString(cfg.Table) {
		return nil, errors.New("pgvector: invalid table name: must match ^[a-zA-Z_][a-zA-Z0-9_]*$")
	}
	if cfg.Dimension <= 0 {
		return nil, errors.New("pgvector: dimension must be > 0")
	}
	return &Store{cfg: cfg}, nil
}

// AutoMigrate 幂等建表和建索引.
func (s *Store) AutoMigrate(ctx context.Context) error {
	ddl := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id       TEXT PRIMARY KEY,
			content  TEXT NOT NULL,
			vector   vector(%d) NOT NULL,
			metadata JSONB
		);
		CREATE INDEX IF NOT EXISTS %s_vec_idx ON %s
			USING ivfflat (vector vector_cosine_ops) WITH (lists = 100);
	`, s.cfg.Table, s.cfg.Dimension, s.cfg.Table, s.cfg.Table)
	_, err := s.cfg.Pool.Exec(ctx, ddl)
	return err
}

// AddDocuments 批量插入文档，ID 冲突时更新.
func (s *Store) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	for _, doc := range docs {
		meta, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("pgvector: marshal metadata for %q: %w", doc.ID, err)
		}
		vecStr := float32SliceToLiteral(doc.Vector)
		sql := fmt.Sprintf(`
			INSERT INTO %s (id, content, vector, metadata)
			VALUES ($1, $2, $3::vector, $4)
			ON CONFLICT (id) DO UPDATE
				SET content=EXCLUDED.content, vector=EXCLUDED.vector, metadata=EXCLUDED.metadata
		`, s.cfg.Table)
		if _, err := s.cfg.Pool.Exec(ctx, sql, doc.ID, doc.Content, vecStr, meta); err != nil {
			return fmt.Errorf("pgvector: insert doc %q: %w", doc.ID, err)
		}
	}
	return nil
}

// SimilaritySearch 基于余弦距离搜索最相似的 k 条文档.
func (s *Store) SimilaritySearch(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
	o := vectorstore.ApplySearchOptions(opts)
	vecStr := float32SliceToLiteral(query)

	var conditions []string
	args := []any{vecStr, k}

	if o.Filter() != nil {
		filterJSON, err := json.Marshal(o.Filter())
		if err != nil {
			return nil, fmt.Errorf("pgvector: marshal filter: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("metadata @> $%d::jsonb", len(args)+1))
		args = append(args, string(filterJSON))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	sql := fmt.Sprintf(`
		SELECT id, content, metadata, 1 - (vector <=> $1::vector) AS score
		FROM %s
		%s
		ORDER BY vector <=> $1::vector
		LIMIT $2
	`, s.cfg.Table, where)

	rows, err := s.cfg.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("pgvector: search: %w", err)
	}
	defer rows.Close()

	var results []vectorstore.SearchResult
	for rows.Next() {
		var id, content string
		var metaBytes []byte
		var scoreF64 float64
		if err := rows.Scan(&id, &content, &metaBytes, &scoreF64); err != nil {
			return nil, fmt.Errorf("pgvector: scan: %w", err)
		}
		score := float32(scoreF64)
		if o.ScoreThreshold() != nil && score < *o.ScoreThreshold() {
			continue
		}
		var meta map[string]any
		if len(metaBytes) > 0 {
			if err := json.Unmarshal(metaBytes, &meta); err != nil {
				return nil, fmt.Errorf("pgvector: unmarshal metadata: %w", err)
			}
		}
		results = append(results, vectorstore.SearchResult{
			Document: vectorstore.Document{ID: id, Content: content, Metadata: meta},
			Score:    score,
		})
	}
	return results, rows.Err()
}

// Delete 根据 ID 列表删除文档.
func (s *Store) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	sql := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", s.cfg.Table, strings.Join(placeholders, ","))
	_, err := s.cfg.Pool.Exec(ctx, sql, args...)
	return err
}

// float32SliceToLiteral 将向量转为 pgvector 字面量 "[1,2,3]".
func float32SliceToLiteral(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// 编译期接口断言.
var _ vectorstore.VectorStore = (*Store)(nil)
