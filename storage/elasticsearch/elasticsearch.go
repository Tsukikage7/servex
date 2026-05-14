// Package elasticsearch 提供 Elasticsearch 客户端封装.
//
// 特性:
//   - 基于官方 go-elasticsearch/v8 实现
//   - 支持索引管理、文档 CRUD、批量操作
//   - 支持搜索查询和聚合分析
//   - 接口化设计，便于测试和替换
//
// 示例:
//
//	client, _ := elasticsearch.NewClient(&elasticsearch.Config{
//	    Addresses: []string{"http://localhost:9200"},
//	})
//	defer client.Close()
//
//	idx := client.Index("my-index")
//	idx.Document().Index(ctx, "1", map[string]any{"name": "John"})
package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 预定义错误.
var (
	// ErrNilConfig 配置为 nil 时返回.
	ErrNilConfig = errors.New("elasticsearch: config is nil")
	// ErrNilLogger 日志记录器为 nil 时返回.
	ErrNilLogger = errors.New("elasticsearch: logger is nil")
	// ErrEmptyAddresses 地址列表为空时返回.
	ErrEmptyAddresses = errors.New("elasticsearch: addresses is empty")
	// ErrIndexNotFound 索引未找到.
	ErrIndexNotFound = errors.New("elasticsearch: index not found")
	// ErrDocumentNotFound 文档未找到.
	ErrDocumentNotFound = errors.New("elasticsearch: document not found")
	// ErrBulkPartialFailure 批量操作部分失败.
	ErrBulkPartialFailure = errors.New("elasticsearch: bulk operation has failures")
	// ErrRequestFailed 请求失败.
	ErrRequestFailed = errors.New("elasticsearch: request failed")
)

// Config Elasticsearch 配置.
type Config struct {
	// Addresses 节点地址列表
	Addresses []string `json:"addresses" yaml:"addresses" mapstructure:"addresses"`
	// Username 用户名
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	// Password 密码
	Password string `json:"-" yaml:"password" mapstructure:"password"`
	// APIKey API 密钥
	APIKey string `json:"-" yaml:"api_key" mapstructure:"api_key"`
	// CloudID Elastic Cloud ID
	CloudID string `json:"cloud_id" yaml:"cloud_id" mapstructure:"cloud_id"`
	// CACert CA 证书内容
	CACert string `json:"ca_cert" yaml:"ca_cert" mapstructure:"ca_cert"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`
	// MaxIdleConnsPerHost 每个节点最大空闲连接数
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host" mapstructure:"max_idle_conns_per_host"`
	// ResponseHeaderTimeout 响应头超时
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout" yaml:"response_header_timeout" mapstructure:"response_header_timeout"`
	// EnableTracing 启用链路追踪
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
	// PingTimeout Ping 超时时间
	PingTimeout time.Duration `json:"ping_timeout" yaml:"ping_timeout" mapstructure:"ping_timeout"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Addresses:             []string{"http://localhost:9200"},
		MaxRetries:            3,
		MaxIdleConnsPerHost:   10,
		ResponseHeaderTimeout: 30 * time.Second,
		PingTimeout:           5 * time.Second,
	}
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if len(c.Addresses) == 0 && c.CloudID == "" {
		return ErrEmptyAddresses
	}
	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if c.MaxRetries == 0 {
		c.MaxRetries = defaults.MaxRetries
	}
	if c.MaxIdleConnsPerHost == 0 {
		c.MaxIdleConnsPerHost = defaults.MaxIdleConnsPerHost
	}
	if c.ResponseHeaderTimeout == 0 {
		c.ResponseHeaderTimeout = defaults.ResponseHeaderTimeout
	}
	if c.PingTimeout == 0 {
		c.PingTimeout = defaults.PingTimeout
	}
}

// Client Elasticsearch 客户端接口.
type Client interface {
	// Index 获取索引操作
	Index(name string) Index
	// Ping 测试连接
	Ping(ctx context.Context) error
	// Close 关闭连接
	Close() error
	// Client 获取原生客户端
	Client() *elasticsearch.Client
}

// Index 索引操作接口.
type Index interface {
	// Create 创建索引
	Create(ctx context.Context, body map[string]any) error
	// Delete 删除索引
	Delete(ctx context.Context) error
	// Exists 检查索引是否存在
	Exists(ctx context.Context) (bool, error)
	// PutMapping 更新映射
	PutMapping(ctx context.Context, body map[string]any) error
	// GetMapping 获取映射
	GetMapping(ctx context.Context) (map[string]any, error)
	// PutSettings 更新设置
	PutSettings(ctx context.Context, body map[string]any) error
	// PutAlias 添加别名
	PutAlias(ctx context.Context, alias string) error
	// DeleteAlias 删除别名
	DeleteAlias(ctx context.Context, alias string) error
	// Document 获取文档操作
	Document() Document
	// Search 获取搜索操作
	Search() Search
}

// Document 文档操作接口.
type Document interface {
	// Index 索引（创建或覆盖）文档
	Index(ctx context.Context, id string, body any) (*IndexResult, error)
	// Get 获取文档
	Get(ctx context.Context, id string) (*GetResult, error)
	// Update 部分更新文档
	Update(ctx context.Context, id string, body any) (*UpdateResult, error)
	// Delete 删除文档
	Delete(ctx context.Context, id string) (*DeleteResult, error)
	// Bulk 批量操作
	Bulk(ctx context.Context, actions []BulkAction) (*BulkResult, error)
	// Exists 检查文档是否存在
	Exists(ctx context.Context, id string) (bool, error)
}

// Search 搜索操作接口.
type Search interface {
	// Query 执行搜索查询
	Query(ctx context.Context, query map[string]any, opts ...SearchOption) (*SearchResult, error)
	// Count 统计匹配文档数
	Count(ctx context.Context, query map[string]any) (int64, error)
	// Aggregate 执行聚合查询
	Aggregate(ctx context.Context, aggs map[string]any, opts ...SearchOption) (*SearchResult, error)
	// Scroll 滚动查询
	Scroll(ctx context.Context, query map[string]any, size int, opts ...SearchOption) (*SearchResult, error)
	// ClearScroll 清除滚动上下文
	ClearScroll(ctx context.Context, scrollID string) error
}

// IndexResult 索引文档结果.
type IndexResult struct {
	ID      string `json:"_id"`
	Version int64  `json:"_version"`
	Result  string `json:"result"`
}

// GetResult 获取文档结果.
type GetResult struct {
	ID     string          `json:"_id"`
	Found  bool            `json:"found"`
	Source json.RawMessage `json:"_source"`
}

// UpdateResult 更新文档结果.
type UpdateResult struct {
	ID      string `json:"_id"`
	Version int64  `json:"_version"`
	Result  string `json:"result"`
}

// DeleteResult 删除文档结果.
type DeleteResult struct {
	ID      string `json:"_id"`
	Version int64  `json:"_version"`
	Result  string `json:"result"`
}

// BulkAction 批量操作项.
type BulkAction struct {
	// Type 操作类型: "index", "create", "update", "delete"
	Type string `json:"type"`
	// Index 目标索引（可选，默认使用当前索引）
	Index string `json:"index,omitempty"`
	// ID 文档 ID
	ID string `json:"id"`
	// Body 文档内容（delete 操作不需要）
	Body any `json:"body,omitempty"`
}

// BulkResult 批量操作结果.
type BulkResult struct {
	Took   int              `json:"took"`
	Errors bool             `json:"errors"`
	Items  []BulkResultItem `json:"items"`
}

// BulkResultItem 批量操作单项结果.
type BulkResultItem struct {
	Index  string       `json:"_index"`
	ID     string       `json:"_id"`
	Status int          `json:"status"`
	Error  *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail ES 错误详情.
type ErrorDetail struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// SearchResult 搜索结果.
type SearchResult struct {
	Took         int                        `json:"took"`
	TotalHits    int64                      `json:"total_hits"`
	MaxScore     float64                    `json:"max_score"`
	Hits         []Hit                      `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations,omitzero"`
	ScrollID     string                     `json:"_scroll_id,omitempty"`
}

// Hit 搜索命中项.
type Hit struct {
	Index     string              `json:"_index"`
	ID        string              `json:"_id"`
	Score     float64             `json:"_score"`
	Source    json.RawMessage     `json:"_source"`
	Highlight map[string][]string `json:"highlight,omitzero"`
}

// SearchOption 搜索选项.
type SearchOption func(*searchOptions)

type searchOptions struct {
	from           int
	size           int
	sort           []map[string]any
	highlight      map[string]any
	sourceIncludes []string
	sourceExcludes []string
	routing        string
	scrollDuration time.Duration
}

// WithFrom 设置起始偏移.
func WithFrom(from int) SearchOption {
	return func(o *searchOptions) { o.from = from }
}

// WithSize 设置返回数量.
func WithSize(size int) SearchOption {
	return func(o *searchOptions) { o.size = size }
}

// WithSort 设置排序.
func WithSort(fields ...map[string]any) SearchOption {
	return func(o *searchOptions) { o.sort = fields }
}

// WithHighlight 设置高亮.
func WithHighlight(highlight map[string]any) SearchOption {
	return func(o *searchOptions) { o.highlight = highlight }
}

// WithSourceIncludes 设置返回字段.
func WithSourceIncludes(fields ...string) SearchOption {
	return func(o *searchOptions) { o.sourceIncludes = fields }
}

// WithSourceExcludes 设置排除字段.
func WithSourceExcludes(fields ...string) SearchOption {
	return func(o *searchOptions) { o.sourceExcludes = fields }
}

// WithRouting 设置路由.
func WithRouting(routing string) SearchOption {
	return func(o *searchOptions) { o.routing = routing }
}

// WithScrollDuration 设置滚动查询的保持时间.
func WithScrollDuration(d time.Duration) SearchOption {
	return func(o *searchOptions) { o.scrollDuration = d }
}

// NewClient 创建 Elasticsearch 客户端.
func NewClient(config *Config, log logger.Logger) (Client, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	config.ApplyDefaults()

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return newESClient(config, log)
}

// MustNewClient 创建 Elasticsearch 客户端，失败时 panic.
func MustNewClient(config *Config, log logger.Logger) Client {
	client, err := NewClient(config, log)
	if err != nil {
		panic(err)
	}
	return client
}
