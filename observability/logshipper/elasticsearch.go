package logshipper

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/storage/elasticsearch"
)

// ElasticsearchSink ES 日志投递，将日志批量写入 Elasticsearch 索引.
type ElasticsearchSink struct {
	client      elasticsearch.Client
	indexPrefix string
	dateSuffix  string
}

// ESOption ElasticsearchSink 选项.
type ESOption func(*ElasticsearchSink)

// WithIndexPrefix 设置索引前缀，默认 "logs-".
func WithIndexPrefix(prefix string) ESOption {
	return func(s *ElasticsearchSink) {
		s.indexPrefix = prefix
	}
}

// WithDateSuffix 设置日期后缀格式（Go time 格式），默认 "2006.01.02"（按日分索引）.
func WithDateSuffix(format string) ESOption {
	return func(s *ElasticsearchSink) {
		s.dateSuffix = format
	}
}

// NewElasticsearchSink 创建 ES 日志投递目标.
func NewElasticsearchSink(client elasticsearch.Client, opts ...ESOption) *ElasticsearchSink {
	s := &ElasticsearchSink{
		client:      client,
		indexPrefix: "logs-",
		dateSuffix:  "2006.01.02",
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// indexName 根据当前时间生成索引名称，例如 "logs-2026.04.05".
func (s *ElasticsearchSink) indexName(t time.Time) string {
	return s.indexPrefix + t.UTC().Format(s.dateSuffix)
}

// entryID 生成文档 ID：时间戳纳秒 + UUID 后缀，确保唯一性.
func entryID(e Entry) string {
	return fmt.Sprintf("%d-%s", e.Timestamp.UnixNano(), uuid.New().String())
}

// Write 将日志条目批量写入 ES，使用 Bulk 操作提升效率.
// 同一批次中所有条目按时间戳分组到对应的日期索引.
func (s *ElasticsearchSink) Write(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	// 按索引名分组，避免跨日期批次跨索引写入问题
	groups := make(map[string][]elasticsearch.BulkAction)
	for _, e := range entries {
		idxName := s.indexName(e.Timestamp)
		groups[idxName] = append(groups[idxName], elasticsearch.BulkAction{
			Type: "index",
			ID:   entryID(e),
			Body: e,
		})
	}

	for idxName, actions := range groups {
		result, err := s.client.Index(idxName).Document().Bulk(ctx, actions)
		if err != nil {
			return fmt.Errorf("logshipper/elasticsearch: bulk write to %s: %w", idxName, err)
		}
		if result != nil && result.Errors {
			return fmt.Errorf("logshipper/elasticsearch: bulk write to %s: %w", idxName, elasticsearch.ErrBulkPartialFailure)
		}
	}
	return nil
}

// Close 关闭 ES sink.
// ES Client 的生命周期由调用者管理，此处不关闭 client.
func (s *ElasticsearchSink) Close() error {
	return nil
}
