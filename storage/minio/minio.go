// Package minio 提供 MinIO 对象存储客户端封装.
//
// 特性:
//   - 基于 MinIO 原生 Go SDK 实现
//   - 支持对象上传、下载、删除、列表、复制
//   - 支持预签名 URL
//   - 支持文件级别上传下载（FPutObject / FGetObject）
//
// 示例:
//
//	client, _ := minio.NewClient(&minio.Config{
//	    Endpoint:  "localhost:9000",
//	    AccessKey: "minioadmin",
//	    SecretKey: "minioadmin",
//	    Bucket:    "my-bucket",
//	})
//
//	// 上传文件
//	client.PutObject(ctx, "path/to/file.txt", reader, size, "application/octet-stream")
//
//	// 获取预签名 URL
//	url, _ := client.PresignGetObject(ctx, "path/to/file.txt", 1*time.Hour)
package minio

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 预定义错误.
var (
	// ErrNilConfig 配置为 nil 时返回.
	ErrNilConfig = errors.New("minio: config is nil")
	// ErrEmptyEndpoint 端点地址为空时返回.
	ErrEmptyEndpoint = errors.New("minio: endpoint is empty")
	// ErrEmptyBucket 桶名为空时返回.
	ErrEmptyBucket = errors.New("minio: bucket is empty")
	// ErrEmptyKey 对象键为空时返回.
	ErrEmptyKey = errors.New("minio: key is empty")
	// ErrObjectNotFound 对象未找到.
	ErrObjectNotFound = errors.New("minio: object not found")
)

// Config MinIO 配置.
type Config struct {
	// Endpoint MinIO 端点地址（不含协议前缀，如 "localhost:9000"）.
	Endpoint string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	// AccessKey 访问密钥 ID.
	AccessKey string `json:"-" yaml:"access_key" mapstructure:"access_key"`
	// SecretKey 访问密钥.
	SecretKey string `json:"-" yaml:"secret_key" mapstructure:"secret_key"`
	// Bucket 默认桶名.
	Bucket string `json:"bucket" yaml:"bucket" mapstructure:"bucket"`
	// UseSSL 是否使用 SSL.
	UseSSL bool `json:"use_ssl" yaml:"use_ssl" mapstructure:"use_ssl"`
	// Region 区域.
	Region string `json:"region" yaml:"region" mapstructure:"region"`
	// ConnectTimeout 连接超时.
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout" mapstructure:"connect_timeout"`
	// EnableTracing 启用链路追踪
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Region:         "us-east-1",
		UseSSL:         false,
		ConnectTimeout: 10 * time.Second,
	}
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return ErrEmptyEndpoint
	}
	if c.Bucket == "" {
		return ErrEmptyBucket
	}
	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if c.Region == "" {
		c.Region = defaults.Region
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = defaults.ConnectTimeout
	}
}

// Option 客户端选项函数.
type Option func(*clientOptions)

type clientOptions struct {
	log logger.Logger
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *clientOptions) {
		o.log = log
	}
}

// Client MinIO 客户端.
type Client struct {
	mc     *minio.Client
	bucket string
	config *Config
	log    logger.Logger
}

// NewClient 创建 MinIO 客户端.
func NewClient(cfg *Config, opts ...Option) (*Client, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	co := &clientOptions{}
	for _, opt := range opts {
		opt(co)
	}

	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    cfg.UseSSL,
		Region:    cfg.Region,
		Transport: minioTransport(cfg.EnableTracing),
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		mc:     mc,
		bucket: cfg.Bucket,
		config: cfg,
		log:    co.log,
	}

	if c.log != nil {
		c.log.Info("minio 连接成功", "endpoint", cfg.Endpoint, "bucket", cfg.Bucket)
	}

	return c, nil
}

// PutObject 上传对象.
func (c *Client) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	if key == "" {
		return minio.UploadInfo{}, ErrEmptyKey
	}
	return c.mc.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

// GetObject 获取对象.
// 注意：调用方必须关闭返回的 *minio.Object 以释放连接资源.
func (c *Client) GetObject(ctx context.Context, key string) (*minio.Object, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	return c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
}

// DeleteObject 删除对象.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	if key == "" {
		return ErrEmptyKey
	}
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// ListObjects 列出指定前缀的对象.
func (c *Client) ListObjects(ctx context.Context, prefix string, recursive bool) <-chan minio.ObjectInfo {
	return c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	})
}

// StatObject 获取对象元信息.
func (c *Client) StatObject(ctx context.Context, key string) (minio.ObjectInfo, error) {
	if key == "" {
		return minio.ObjectInfo{}, ErrEmptyKey
	}
	info, err := c.mc.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" {
			return minio.ObjectInfo{}, ErrObjectNotFound
		}
		return minio.ObjectInfo{}, err
	}
	return info, nil
}

// PresignGetObject 生成下载预签名 URL.
func (c *Client) PresignGetObject(ctx context.Context, key string, expires time.Duration) (*url.URL, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	return c.mc.PresignedGetObject(ctx, c.bucket, key, expires, nil)
}

// PresignPutObject 生成上传预签名 URL.
func (c *Client) PresignPutObject(ctx context.Context, key string, expires time.Duration) (*url.URL, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	return c.mc.PresignedPutObject(ctx, c.bucket, key, expires)
}

// BucketExists 检查桶是否存在.
func (c *Client) BucketExists(ctx context.Context) (bool, error) {
	return c.mc.BucketExists(ctx, c.bucket)
}

// MakeBucket 创建桶.
func (c *Client) MakeBucket(ctx context.Context) error {
	return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{
		Region: c.config.Region,
	})
}

// CopyObject 复制对象.
func (c *Client) CopyObject(ctx context.Context, srcKey, destKey string) (minio.UploadInfo, error) {
	if srcKey == "" || destKey == "" {
		return minio.UploadInfo{}, ErrEmptyKey
	}
	src := minio.CopySrcOptions{
		Bucket: c.bucket,
		Object: srcKey,
	}
	dst := minio.CopyDestOptions{
		Bucket: c.bucket,
		Object: destKey,
	}
	return c.mc.CopyObject(ctx, dst, src)
}

// FGetObject 下载对象到本地文件.
func (c *Client) FGetObject(ctx context.Context, key, filePath string) error {
	if key == "" {
		return ErrEmptyKey
	}
	return c.mc.FGetObject(ctx, c.bucket, key, filePath, minio.GetObjectOptions{})
}

// FPutObject 从本地文件上传对象.
func (c *Client) FPutObject(ctx context.Context, key, filePath, contentType string) (minio.UploadInfo, error) {
	if key == "" {
		return minio.UploadInfo{}, ErrEmptyKey
	}
	return c.mc.FPutObject(ctx, c.bucket, key, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

// minioTransport 返回 MinIO 使用的 HTTP Transport.
// 启用链路追踪时，用 otelhttp 包装默认 Transport.
func minioTransport(enableTracing bool) http.RoundTripper {
	tp := http.DefaultTransport.(*http.Transport).Clone()
	if enableTracing {
		return otelhttp.NewTransport(tp, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return "MinIO " + r.Method + " " + r.URL.Path
		}))
	}
	return tp
}
