// Package rdbms 提供数据库连接和管理功能.
package rdbms

import (
	"errors"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 支持的驱动类型.
const (
	// DriverMySQL MySQL 驱动.
	DriverMySQL = "mysql"
	// DriverPostgres PostgreSQL 驱动.
	DriverPostgres = "postgres"
	// DriverPostgreSQL PostgreSQL 驱动（别名）.
	DriverPostgreSQL = "postgresql"
	// DriverSQLite SQLite 驱动.
	DriverSQLite = "sqlite"
	// DriverSQLite3 SQLite3 驱动（别名）.
	DriverSQLite3 = "sqlite3"
)

// 支持的 ORM 类型.
const (
	// TypeGORM GORM ORM 类型.
	TypeGORM = "gorm"
)

// 预定义错误.
var (
	// ErrNilConfig 配置为空.
	ErrNilConfig = errors.New("database: 配置为空")
	// ErrNilLogger 日志记录器为空.
	ErrNilLogger = errors.New("database: 日志记录器为空")
	// ErrEmptyDriver 驱动类型为空.
	ErrEmptyDriver = errors.New("database: 驱动类型为空")
	// ErrEmptyDSN 连接字符串为空.
	ErrEmptyDSN = errors.New("database: 连接字符串为空")
	// ErrUnsupportedDriver 不支持的驱动类型.
	ErrUnsupportedDriver = errors.New("database: 不支持的驱动类型")
	// ErrUnsupportedType 不支持的 ORM 类型.
	ErrUnsupportedType = errors.New("database: 不支持的 ORM 类型")
	// ErrRegisterTracingPlugin 注册追踪插件失败.
	ErrRegisterTracingPlugin = errors.New("database: 注册追踪插件失败")
	// ErrNilDatabase 数据库为空.
	ErrNilDatabase = errors.New("database: 数据库为空")
	// ErrNotGORMDatabase 数据库不是 GORM 实现.
	ErrNotGORMDatabase = errors.New("database: 无法提取 *gorm.DB，请确保使用 GORM 类型的数据库")
)

// Config 数据库配置.
type Config struct {
	// Type ORM 类型，目前支持 "gorm"
	Type string `json:"type" toml:"type" yaml:"type" mapstructure:"type"`

	// Driver 数据库驱动类型：mysql, postgres, sqlite
	Driver string `json:"driver" toml:"driver" yaml:"driver" mapstructure:"driver"`

	// DSN 数据库连接字符串
	// json:"-" 防止序列化时泄露连接凭据
	DSN string `json:"-" toml:"dsn" yaml:"dsn" mapstructure:"dsn"`

	// AutoMigrate 是否自动迁移表结构
	AutoMigrate bool `json:"auto_migrate" toml:"auto_migrate" yaml:"auto_migrate" mapstructure:"auto_migrate"`

	// Pool 连接池配置
	Pool PoolConfig `json:"pool" toml:"pool" yaml:"pool" mapstructure:"pool"`

	// SlowThreshold 慢查询阈值
	SlowThreshold time.Duration `json:"slow_threshold" toml:"slow_threshold" yaml:"slow_threshold" mapstructure:"slow_threshold"`

	// LogLevel 日志级别: silent, error, warn, info
	LogLevel string `json:"log_level" toml:"log_level" yaml:"log_level" mapstructure:"log_level"`

	// EnableTracing 启用链路追踪
	EnableTracing bool `json:"enable_tracing" toml:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// PoolConfig 连接池配置.
type PoolConfig struct {
	// MaxOpen 最大打开连接数
	MaxOpen int `json:"max_open" toml:"max_open" yaml:"max_open" mapstructure:"max_open"`

	// MaxIdle 最大空闲连接数
	MaxIdle int `json:"max_idle" toml:"max_idle" yaml:"max_idle" mapstructure:"max_idle"`

	// MaxLifetime 连接最大生命周期
	MaxLifetime time.Duration `json:"max_lifetime" toml:"max_lifetime" yaml:"max_lifetime" mapstructure:"max_lifetime"`

	// MaxIdleTime 空闲连接最大存活时间
	MaxIdleTime time.Duration `json:"max_idle_time" toml:"max_idle_time" yaml:"max_idle_time" mapstructure:"max_idle_time"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Type:          TypeGORM,
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      "info",
		Pool: PoolConfig{
			MaxOpen:     100,
			MaxIdle:     10,
			MaxLifetime: time.Hour,
			MaxIdleTime: 10 * time.Minute,
		},
	}
}

// DefaultPoolConfig 返回默认连接池配置.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpen:     100,
		MaxIdle:     10,
		MaxLifetime: time.Hour,
		MaxIdleTime: 10 * time.Minute,
	}
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c.Driver == "" {
		return ErrEmptyDriver
	}
	if c.DSN == "" {
		return ErrEmptyDSN
	}
	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	if c.Type == "" {
		c.Type = TypeGORM
	}
	if c.SlowThreshold == 0 {
		c.SlowThreshold = 200 * time.Millisecond
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.Pool.MaxOpen == 0 {
		c.Pool.MaxOpen = 100
	}
	if c.Pool.MaxIdle == 0 {
		c.Pool.MaxIdle = 10
	}
	if c.Pool.MaxLifetime == 0 {
		c.Pool.MaxLifetime = time.Hour
	}
	if c.Pool.MaxIdleTime == 0 {
		c.Pool.MaxIdleTime = 10 * time.Minute
	}
}

// Database 数据库操作接口.
type Database interface {
	// AutoMigrate 自动迁移表结构
	AutoMigrate(models ...any) error

	// DB 获取底层数据库实例
	DB() any

	// Close 关闭数据库连接
	Close() error
}

// NewDatabase 创建数据库连接.
func NewDatabase(config *Config, log logger.Logger) (Database, error) {
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

	switch config.Type {
	case TypeGORM:
		return newGORMDatabase(config, log)
	default:
		return nil, ErrUnsupportedType
	}
}

// NewDatabaseWithCleanup 创建数据库连接，并返回用于依赖注入生命周期的清理函数.
func NewDatabaseWithCleanup(config *Config, log logger.Logger) (Database, func(), error) {
	db, err := NewDatabase(config, log)
	if err != nil {
		return nil, nil, err
	}
	return db, func() { _ = db.Close() }, nil
}

// MustNewDatabase 创建数据库连接，失败时 panic.
func MustNewDatabase(config *Config, log logger.Logger) Database {
	db, err := NewDatabase(config, log)
	if err != nil {
		panic(err)
	}
	return db
}
