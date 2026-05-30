// Package mongodb 提供 MongoDB 客户端封装.
//
// 特性:
//   - 基于官方 mongo-driver 实现
//   - 支持连接池配置
//   - 支持链路追踪
//   - 支持常用 CRUD 操作封装
//
// 示例:
//
//	client, _ := mongodb.NewClient(&mongodb.Config{
//	    URI:      "mongodb://localhost:27017",
//	    Database: "mydb",
//	})
//	defer client.Close()
//
//	// 获取集合
//	coll := client.Collection("users")
//	coll.InsertOne(ctx, bson.M{"name": "John"})
package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 预定义错误.
var (
	// ErrNilConfig 配置为 nil 时返回.
	ErrNilConfig = errors.New("mongodb: config is nil")
	// ErrNilLogger 日志记录器为 nil 时返回.
	ErrNilLogger = errors.New("mongodb: logger is nil")
	// ErrEmptyURI URI 为空时返回.
	ErrEmptyURI = errors.New("mongodb: URI is empty")
	// ErrEmptyDatabase 数据库名为空时返回.
	ErrEmptyDatabase = errors.New("mongodb: database name is empty")
	// ErrNotConnected 未连接时返回.
	ErrNotConnected = errors.New("mongodb: not connected")
	// ErrNoDocuments 无匹配文档时返回.
	ErrNoDocuments = mongo.ErrNoDocuments
)

// Config MongoDB 配置.
type Config struct {
	// URI 连接字符串
	URI string `json:"-" yaml:"uri" mapstructure:"uri"`
	// Database 数据库名
	Database string `json:"database" yaml:"database" mapstructure:"database"`
	// ConnectTimeout 连接超时
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout" mapstructure:"connect_timeout"`
	// ServerSelectionTimeout 服务器选择超时
	ServerSelectionTimeout time.Duration `json:"server_selection_timeout" yaml:"server_selection_timeout" mapstructure:"server_selection_timeout"`
	// SocketTimeout Socket 超时
	SocketTimeout time.Duration `json:"socket_timeout" yaml:"socket_timeout" mapstructure:"socket_timeout"`
	// MaxPoolSize 最大连接池大小
	MaxPoolSize uint64 `json:"max_pool_size" yaml:"max_pool_size" mapstructure:"max_pool_size"`
	// MinPoolSize 最小连接池大小
	MinPoolSize uint64 `json:"min_pool_size" yaml:"min_pool_size" mapstructure:"min_pool_size"`
	// MaxConnIdleTime 连接最大空闲时间
	MaxConnIdleTime time.Duration `json:"max_conn_idle_time" yaml:"max_conn_idle_time" mapstructure:"max_conn_idle_time"`
	// ReplicaSet 副本集名称
	ReplicaSet string `json:"replica_set" yaml:"replica_set" mapstructure:"replica_set"`
	// Direct 是否直连单节点
	Direct bool `json:"direct" yaml:"direct" mapstructure:"direct"`
	// EnableTracing 启用链路追踪
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		ConnectTimeout:         10 * time.Second,
		ServerSelectionTimeout: 5 * time.Second,
		SocketTimeout:          30 * time.Second,
		MaxPoolSize:            100,
		MinPoolSize:            5,
		MaxConnIdleTime:        10 * time.Minute,
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
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = defaults.ConnectTimeout
	}
	if c.ServerSelectionTimeout == 0 {
		c.ServerSelectionTimeout = defaults.ServerSelectionTimeout
	}
	if c.SocketTimeout == 0 {
		c.SocketTimeout = defaults.SocketTimeout
	}
	if c.MaxPoolSize == 0 {
		c.MaxPoolSize = defaults.MaxPoolSize
	}
	if c.MinPoolSize == 0 {
		c.MinPoolSize = defaults.MinPoolSize
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = defaults.MaxConnIdleTime
	}
}

// Client MongoDB 客户端接口.
type Client interface {
	// Database 获取数据库
	Database(name ...string) Database
	// Collection 获取集合使用默认数据库
	Collection(name string) Collection
	// Ping 测试连接
	Ping(ctx context.Context) error
	// Close 关闭连接
	Close(ctx context.Context) error
	// Client 获取原生客户端
	Client() *mongo.Client
	// StartSession 开始会话
	StartSession(opts ...options.Lister[options.SessionOptions]) (*mongo.Session, error)
	// UseSession 使用会话执行操作
	UseSession(ctx context.Context, fn func(ctx context.Context) error) error
	// UseTransaction 使用事务执行操作
	UseTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Database MongoDB 数据库接口.
type Database interface {
	// Collection 获取集合
	Collection(name string) Collection
	// Name 返回数据库名
	Name() string
	// Drop 删除数据库
	Drop(ctx context.Context) error
	// ListCollectionNames 列出所有集合名
	ListCollectionNames(ctx context.Context, filter any) ([]string, error)
	// Database 获取原生数据库
	Database() *mongo.Database
}

// Collection MongoDB 集合接口.
type Collection interface {
	// Name 返回集合名
	Name() string

	// InsertOne 插入单个文档
	InsertOne(ctx context.Context, document any) (*InsertOneResult, error)
	// InsertMany 插入多个文档
	InsertMany(ctx context.Context, documents []any) (*InsertManyResult, error)

	// FindOne 查询单个文档
	FindOne(ctx context.Context, filter any, opts ...FindOneOption) SingleResult
	// Find 查询多个文档
	Find(ctx context.Context, filter any, opts ...FindOption) (Cursor, error)
	// CountDocuments 统计文档数量
	CountDocuments(ctx context.Context, filter any) (int64, error)
	// EstimatedDocumentCount 估算文档数量
	EstimatedDocumentCount(ctx context.Context) (int64, error)
	// Distinct 获取字段的不同值
	Distinct(ctx context.Context, fieldName string, filter any) ([]any, error)

	// UpdateOne 更新单个文档
	UpdateOne(ctx context.Context, filter, update any) (*UpdateResult, error)
	// UpdateMany 更新多个文档
	UpdateMany(ctx context.Context, filter, update any) (*UpdateResult, error)
	// ReplaceOne 替换单个文档
	ReplaceOne(ctx context.Context, filter, replacement any) (*UpdateResult, error)

	// DeleteOne 删除单个文档
	DeleteOne(ctx context.Context, filter any) (*DeleteResult, error)
	// DeleteMany 删除多个文档
	DeleteMany(ctx context.Context, filter any) (*DeleteResult, error)

	// FindOneAndUpdate 查询并更新
	FindOneAndUpdate(ctx context.Context, filter, update any) SingleResult
	// FindOneAndReplace 查询并替换
	FindOneAndReplace(ctx context.Context, filter, replacement any) SingleResult
	// FindOneAndDelete 查询并删除
	FindOneAndDelete(ctx context.Context, filter any) SingleResult

	// Aggregate 聚合查询
	Aggregate(ctx context.Context, pipeline any) (Cursor, error)

	// CreateIndex 创建索引
	CreateIndex(ctx context.Context, model IndexModel) (string, error)
	// CreateIndexes 创建多个索引
	CreateIndexes(ctx context.Context, models []IndexModel) ([]string, error)
	// DropIndex 删除索引
	DropIndex(ctx context.Context, name string) error
	// DropAllIndexes 删除所有索引
	DropAllIndexes(ctx context.Context) error
	// ListIndexes 列出所有索引
	ListIndexes(ctx context.Context) (Cursor, error)

	// Drop 删除集合
	Drop(ctx context.Context) error

	// Collection 获取原生集合
	Collection() *mongo.Collection
}

// SingleResult 单个查询结果.
type SingleResult interface {
	Decode(v any) error
	Err() error
}

// Cursor 游标.
type Cursor interface {
	Next(ctx context.Context) bool
	Decode(v any) error
	All(ctx context.Context, results any) error
	Close(ctx context.Context) error
	Err() error
}

// InsertOneResult 插入单个文档结果.
type InsertOneResult struct {
	InsertedID any
}

// InsertManyResult 插入多个文档结果.
type InsertManyResult struct {
	InsertedIDs []any
}

// UpdateResult 更新结果.
type UpdateResult struct {
	MatchedCount  int64
	ModifiedCount int64
	UpsertedCount int64
	UpsertedID    any
}

// DeleteResult 删除结果.
type DeleteResult struct {
	DeletedCount int64
}

// IndexModel 索引模型.
type IndexModel struct {
	Keys    any
	Options *IndexOptions
}

// IndexOptions 索引选项.
type IndexOptions struct {
	Name               string
	Unique             bool
	Background         bool
	Sparse             bool
	ExpireAfterSeconds int32
}

// FindOneOption 查询单个文档选项.
type FindOneOption func(*findOneOptions)

type findOneOptions struct {
	projection any
	sort       any
	skip       int64
}

// WithProjection 设置投影.
func WithProjection(projection any) FindOneOption {
	return func(o *findOneOptions) {
		o.projection = projection
	}
}

// WithSort 设置排序.
func WithSort(sort any) FindOneOption {
	return func(o *findOneOptions) {
		o.sort = sort
	}
}

// FindOption 查询多个文档选项.
type FindOption func(*findOptions)

type findOptions struct {
	projection any
	sort       any
	skip       int64
	limit      int64
}

// WithFindProjection 设置投影.
func WithFindProjection(projection any) FindOption {
	return func(o *findOptions) {
		o.projection = projection
	}
}

// WithFindSort 设置排序.
func WithFindSort(sort any) FindOption {
	return func(o *findOptions) {
		o.sort = sort
	}
}

// WithFindSkip 设置跳过数量.
func WithFindSkip(skip int64) FindOption {
	return func(o *findOptions) {
		o.skip = skip
	}
}

// WithFindLimit 设置限制数量.
func WithFindLimit(limit int64) FindOption {
	return func(o *findOptions) {
		o.limit = limit
	}
}

// NewClient 创建 MongoDB 客户端.
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

	return newMongoClient(config, log)
}

// MustNewClient 创建 MongoDB 客户端，失败时 panic.
func MustNewClient(config *Config, log logger.Logger) Client {
	client, err := NewClient(config, log)
	if err != nil {
		panic(err)
	}
	return client
}

// D BSON D 类型别名.
type D = bson.D

// M BSON M 类型别名.
type M = bson.M

// A BSON A 类型别名.
type A = bson.A

// E BSON E 类型别名.
type E = bson.E

// ObjectID ObjectID 类型别名.
type ObjectID = bson.ObjectID

// NewObjectID 创建新的 ObjectID.
func NewObjectID() ObjectID {
	return bson.NewObjectID()
}

// ObjectIDFromHex 从十六进制字符串创建 ObjectID.
func ObjectIDFromHex(hex string) (ObjectID, error) {
	return bson.ObjectIDFromHex(hex)
}
