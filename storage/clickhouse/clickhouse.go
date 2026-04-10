// Package clickhouse 提供 ClickHouse 客户端封装.
//
// 特性:
//   - 基于 clickhouse-go/v2 原生协议实现
//   - 支持连接池配置
//   - 支持 LZ4/ZSTD 压缩
//   - 支持批量写入
//
// 示例:
//
//	client, _ := clickhouse.NewClient(&clickhouse.Config{
//	    Addrs:    []string{"localhost:9000"},
//	    Database: "default",
//	})
//	defer client.Close()
//
//	// 执行查询
//	rows, _ := client.Query(ctx, "SELECT 1")
package clickhouse

import (
	"context"
	"time"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/Tsukikage7/servex/observability/logger"
)

// Config ClickHouse 配置.
type Config struct {
	// Addrs 连接地址列表
	Addrs []string `json:"addrs" yaml:"addrs" mapstructure:"addrs"`
	// Database 数据库名
	Database string `json:"database" yaml:"database" mapstructure:"database"`
	// Username 用户名
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	// Password 密码
	Password string `json:"-" yaml:"password" mapstructure:"password"`
	// MaxOpenConns 最大打开连接数
	MaxOpenConns int `json:"max_open_conns" yaml:"max_open_conns" mapstructure:"max_open_conns"`
	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	// DialTimeout 连接超时
	DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout" mapstructure:"dial_timeout"`
	// Compression 压缩方式，支持 "lz4"、"zstd"、"none"
	Compression string `json:"compression" yaml:"compression" mapstructure:"compression"`
	// EnableTracing 启用链路追踪
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Addrs:        []string{"localhost:9000"},
		Database:     "default",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		DialTimeout:  10 * time.Second,
		Compression:  "lz4",
	}
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if len(c.Addrs) == 0 {
		return ErrEmptyAddrs
	}
	if c.Database == "" {
		return ErrEmptyDatabase
	}
	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if len(c.Addrs) == 0 {
		c.Addrs = defaults.Addrs
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = defaults.MaxOpenConns
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = defaults.MaxIdleConns
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaults.DialTimeout
	}
	if c.Compression == "" {
		c.Compression = defaults.Compression
	}
}

// Client ClickHouse 客户端接口.
type Client interface {
	// Exec 执行语句（DDL / INSERT 等不返回行的操作）
	Exec(ctx context.Context, query string, args ...any) error
	// Query 执行查询，返回多行结果
	Query(ctx context.Context, query string, args ...any) (driver.Rows, error)
	// QueryRow 执行查询，返回单行结果
	QueryRow(ctx context.Context, query string, args ...any) driver.Row
	// Select 执行查询并将结果扫描到 dest 切片
	Select(ctx context.Context, dest any, query string, args ...any) error
	// PrepareBatch 准备批量写入
	PrepareBatch(ctx context.Context, query string) (driver.Batch, error)
	// Ping 测试连接
	Ping(ctx context.Context) error
	// Close 关闭连接
	Close() error
	// Conn 获取原生连接
	Conn() driver.Conn
}

// NewClient 创建 ClickHouse 客户端.
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

	return newCHClient(config, log)
}

// MustNewClient 创建 ClickHouse 客户端，失败时 panic.
func MustNewClient(config *Config, log logger.Logger) Client {
	client, err := NewClient(config, log)
	if err != nil {
		panic(err)
	}
	return client
}
