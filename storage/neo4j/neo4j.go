// Package neo4j 提供 Neo4j 图数据库客户端封装.
//
// 特性:
//   - 基于官方 neo4j-go-driver/v5 实现
//   - 支持连接池配置
//   - 支持读写事务
//   - 支持便捷的 Cypher 查询执行
//
// 示例:
//
//	client, _ := neo4j.NewClient(&neo4j.Config{
//	    URI:      "neo4j://localhost:7687",
//	    Database: "neo4j",
//	})
//	defer client.Close(ctx)
//
//	// 写事务
//	client.WriteTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
//	    _, err := tx.Run(ctx, "CREATE (n:Person {name: $name})", map[string]any{"name": "Alice"})
//	    return nil, err
//	})
package neo4j

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 预定义错误.
var (
	// ErrNilConfig 配置为 nil 时返回.
	ErrNilConfig = errors.New("neo4j: config is nil")
	// ErrNilLogger 日志记录器为 nil 时返回.
	ErrNilLogger = errors.New("neo4j: logger is nil")
	// ErrEmptyURI URI 为空时返回.
	ErrEmptyURI = errors.New("neo4j: URI is empty")
	// ErrEmptyDatabase 数据库名为空时返回.
	ErrEmptyDatabase = errors.New("neo4j: database name is empty")
	// ErrNotConnected 未连接时返回.
	ErrNotConnected = errors.New("neo4j: not connected")
)

// Config Neo4j 配置.
type Config struct {
	// URI 连接字符串.
	URI string `json:"uri" yaml:"uri" mapstructure:"uri"`
	// Username 用户名.
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	// Password 密码.
	Password string `json:"-" yaml:"password" mapstructure:"password"`
	// Database 数据库名.
	Database string `json:"database" yaml:"database" mapstructure:"database"`
	// MaxConnectionPoolSize 最大连接池大小.
	MaxConnectionPoolSize int `json:"max_connection_pool_size" yaml:"max_connection_pool_size" mapstructure:"max_connection_pool_size"`
	// ConnectionAcquisitionTimeout 连接获取超时.
	ConnectionAcquisitionTimeout time.Duration `json:"connection_acquisition_timeout" yaml:"connection_acquisition_timeout" mapstructure:"connection_acquisition_timeout"`
	// MaxTransactionRetryTime 最大事务重试时间.
	MaxTransactionRetryTime time.Duration `json:"max_transaction_retry_time" yaml:"max_transaction_retry_time" mapstructure:"max_transaction_retry_time"`
	// Encrypted 是否启用加密连接.
	Encrypted bool `json:"encrypted" yaml:"encrypted" mapstructure:"encrypted"`
	// EnableTracing 启用链路追踪.
	// TODO: 待 neo4j-go-driver OTEL 集成库成熟后启用
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Database:                     "neo4j",
		MaxConnectionPoolSize:        100,
		ConnectionAcquisitionTimeout: 60 * time.Second,
		MaxTransactionRetryTime:      30 * time.Second,
	}
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c.URI == "" {
		return ErrEmptyURI
	}
	if c.Database == "" {
		return ErrEmptyDatabase
	}
	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if c.Database == "" {
		c.Database = defaults.Database
	}
	if c.MaxConnectionPoolSize == 0 {
		c.MaxConnectionPoolSize = defaults.MaxConnectionPoolSize
	}
	if c.ConnectionAcquisitionTimeout == 0 {
		c.ConnectionAcquisitionTimeout = defaults.ConnectionAcquisitionTimeout
	}
	if c.MaxTransactionRetryTime == 0 {
		c.MaxTransactionRetryTime = defaults.MaxTransactionRetryTime
	}
}

// Option 客户端选项.
type Option func(*Client)

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(c *Client) {
		c.log = log
	}
}

// Record Cypher 查询返回的单条记录.
type Record struct {
	// Keys 字段名列表.
	Keys []string
	// Values 字段值列表.
	Values []any
}

// GetByIndex 按索引获取字段值.
func (r *Record) GetByIndex(index int) any {
	if index < 0 || index >= len(r.Values) {
		return nil
	}
	return r.Values[index]
}

// Get 按字段名获取值.
func (r *Record) Get(key string) (any, bool) {
	for i, k := range r.Keys {
		if k == key {
			if i < len(r.Values) {
				return r.Values[i], true
			}
			return nil, false
		}
	}
	return nil, false
}

// Client Neo4j 客户端.
type Client struct {
	driver   neo4j.DriverWithContext
	database string
	log      logger.Logger
}

// NewClient 创建 Neo4j 客户端.
func NewClient(cfg *Config, opts ...Option) (*Client, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// 构建认证
	var auth neo4j.AuthToken
	if cfg.Username != "" {
		auth = neo4j.BasicAuth(cfg.Username, cfg.Password, "")
	} else {
		auth = neo4j.NoAuth()
	}

	// 构建驱动配置
	configurers := []func(*neo4j.Config){
		func(c *neo4j.Config) {
			c.MaxConnectionPoolSize = cfg.MaxConnectionPoolSize
			c.ConnectionAcquisitionTimeout = cfg.ConnectionAcquisitionTimeout
			c.MaxTransactionRetryTime = cfg.MaxTransactionRetryTime
		},
	}

	driver, err := neo4j.NewDriverWithContext(cfg.URI, auth, configurers...)
	if err != nil {
		return nil, err
	}

	client := &Client{
		driver:   driver,
		database: cfg.Database,
	}

	for _, opt := range opts {
		opt(client)
	}

	// 验证连通性
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, err
	}

	if client.log != nil {
		client.log.Info("neo4j 连接成功", "host", maskNeo4jURI(cfg.URI), "database", cfg.Database)
	}

	return client, nil
}

// Close 关闭连接.
func (c *Client) Close(ctx context.Context) error {
	if c.driver == nil {
		return ErrNotConnected
	}
	if c.log != nil {
		c.log.Info("neo4j 连接关闭")
	}
	return c.driver.Close(ctx)
}

// maskNeo4jURI 遮盖 URI 中的认证信息，仅保留 host 部分.
func maskNeo4jURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return "***"
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// Driver 获取原生驱动.
func (c *Client) Driver() neo4j.DriverWithContext {
	return c.driver
}

// Session 获取会话.
func (c *Client) Session(ctx context.Context, mode neo4j.AccessMode) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   mode,
		DatabaseName: c.database,
	})
}

// ReadTransaction 执行读事务.
func (c *Client) ReadTransaction(ctx context.Context, fn func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	session := c.Session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx) //nolint:errcheck

	return session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return fn(tx)
	})
}

// WriteTransaction 执行写事务.
func (c *Client) WriteTransaction(ctx context.Context, fn func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	session := c.Session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx) //nolint:errcheck

	return session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return fn(tx)
	})
}

// Run 执行 Cypher 查询并返回结果.
// 注意：使用 AccessModeWrite 以支持读写操作.
func (c *Client) Run(ctx context.Context, cypher string, params map[string]any) ([]*Record, error) {
	session := c.Session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx) //nolint:errcheck // session 关闭错误不影响查询结果

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var records []*Record
	for result.Next(ctx) {
		rec := result.Record()
		records = append(records, &Record{
			Keys:   rec.Keys,
			Values: rec.Values,
		})
	}

	if err := result.Err(); err != nil {
		return nil, err
	}

	return records, nil
}
