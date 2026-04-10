// Package s3 提供 S3 兼容对象存储客户端封装.
//
// 特性:
//   - 基于 AWS SDK v2 实现
//   - 兼容 MinIO、阿里云 OSS、腾讯云 COS 等 S3 兼容存储
//   - 支持分片上传、断点续传
//   - 支持预签名 URL
//
// 示例:
//
//	client, _ := s3.NewClient(&s3.Config{
//	    Endpoint:  "http://localhost:9000",
//	    Region:    "us-east-1",
//	    AccessKey: "minioadmin",
//	    SecretKey: "minioadmin",
//	    Bucket:    "my-bucket",
//	})
//
//	// 上传文件
//	client.PutObject(ctx, "path/to/file.txt", reader, size)
//
//	// 获取预签名 URL
//	url, _ := client.PresignGetObject(ctx, "path/to/file.txt", 1*time.Hour)
package s3

import (
	"context"
	"errors"
	"io"
	"time"
)

// 预定义错误.
var (
	// ErrNilConfig 配置为 nil 时返回.
	ErrNilConfig = errors.New("s3: config is nil")
	// ErrNilLogger 日志记录器为 nil 时返回.
	ErrNilLogger = errors.New("s3: logger is nil")
	// ErrEmptyEndpoint 端点地址为空时返回.
	ErrEmptyEndpoint = errors.New("s3: endpoint is empty")
	// ErrEmptyBucket 桶名为空时返回.
	ErrEmptyBucket = errors.New("s3: bucket is empty")
	// ErrEmptyKey 对象键为空时返回.
	ErrEmptyKey = errors.New("s3: key is empty")
	// ErrObjectNotFound 对象未找到.
	ErrObjectNotFound = errors.New("s3: object not found")
)

// Config S3 配置.
type Config struct {
	// Endpoint S3 端点地址
	Endpoint string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	// Region 区域
	Region string `json:"region" yaml:"region" mapstructure:"region"`
	// AccessKey 访问密钥 ID
	AccessKey string `json:"-" yaml:"access_key" mapstructure:"access_key"`
	// SecretKey 访问密钥
	SecretKey string `json:"-" yaml:"secret_key" mapstructure:"secret_key"`
	// Bucket 默认桶名
	Bucket string `json:"bucket" yaml:"bucket" mapstructure:"bucket"`
	// UseSSL 是否使用 SSL
	UseSSL bool `json:"use_ssl" yaml:"use_ssl" mapstructure:"use_ssl"`
	// UsePathStyle 是否使用路径风格（MinIO 需要）
	UsePathStyle bool `json:"use_path_style" yaml:"use_path_style" mapstructure:"use_path_style"`
	// ConnectTimeout 连接超时
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout" mapstructure:"connect_timeout"`
	// RequestTimeout 请求超时
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout" mapstructure:"request_timeout"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`
	// PartSize 分片大小（字节）
	PartSize int64 `json:"part_size" yaml:"part_size" mapstructure:"part_size"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Region:         "us-east-1",
		UseSSL:         true,
		UsePathStyle:   false,
		ConnectTimeout: 10 * time.Second,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		PartSize:       5 * 1024 * 1024, // 5MB
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
	if c.RequestTimeout == 0 {
		c.RequestTimeout = defaults.RequestTimeout
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaults.MaxRetries
	}
	if c.PartSize == 0 {
		c.PartSize = defaults.PartSize
	}
}

// Client S3 客户端接口.
type Client interface {
	// Bucket 操作
	// CreateBucket 创建桶
	CreateBucket(ctx context.Context, bucket string) error
	// DeleteBucket 删除桶
	DeleteBucket(ctx context.Context, bucket string) error
	// BucketExists 检查桶是否存在
	BucketExists(ctx context.Context, bucket string) (bool, error)
	// ListBuckets 列出所有桶
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	// Object 操作
	// PutObject 上传对象
	PutObject(ctx context.Context, key string, reader io.Reader, size int64, opts ...PutOption) (*PutObjectResult, error)
	// GetObject 获取对象
	GetObject(ctx context.Context, key string) (*Object, error)
	// DeleteObject 删除对象
	DeleteObject(ctx context.Context, key string) error
	// DeleteObjects 批量删除对象
	DeleteObjects(ctx context.Context, keys []string) error
	// CopyObject 复制对象
	CopyObject(ctx context.Context, srcKey, destKey string) error
	// HeadObject 获取对象元信息
	HeadObject(ctx context.Context, key string) (*ObjectInfo, error)
	// ObjectExists 检查对象是否存在
	ObjectExists(ctx context.Context, key string) (bool, error)
	// ListObjects 列出对象
	ListObjects(ctx context.Context, prefix string, opts ...ListOption) (*ListObjectsResult, error)

	// 分片上传
	// CreateMultipartUpload 创建分片上传
	CreateMultipartUpload(ctx context.Context, key string, opts ...PutOption) (*MultipartUpload, error)
	// UploadPart 上传分片
	UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int, reader io.Reader, size int64) (*UploadPartResult, error)
	// CompleteMultipartUpload 完成分片上传
	CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload, parts []CompletedPart) (*PutObjectResult, error)
	// AbortMultipartUpload 取消分片上传
	AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error

	// 预签名 URL
	// PresignGetObject 生成下载预签名 URL
	PresignGetObject(ctx context.Context, key string, expires time.Duration) (string, error)
	// PresignPutObject 生成上传预签名 URL
	PresignPutObject(ctx context.Context, key string, expires time.Duration) (string, error)

	// 工具方法
	// Upload 智能上传（自动选择普通/分片上传）
	Upload(ctx context.Context, key string, reader io.Reader, size int64, opts ...PutOption) (*PutObjectResult, error)
	// Download 下载到 Writer
	Download(ctx context.Context, key string, writer io.Writer) (int64, error)

	// UseBucket 使用指定桶
	UseBucket(bucket string) Client
	// Close 关闭客户端
	Close() error
}

// BucketInfo 桶信息.
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// Object 对象.
type Object struct {
	Key           string
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	ETag          string
	LastModified  time.Time
	Metadata      map[string]string
}

// ObjectInfo 对象元信息.
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	ContentType  string
	LastModified time.Time
	StorageClass string
	Metadata     map[string]string
}

// ListObjectsResult 列出对象结果.
type ListObjectsResult struct {
	Objects               []ObjectInfo
	Prefixes              []string
	IsTruncated           bool
	NextMarker            string
	NextContinuationToken string
}

// PutObjectResult 上传对象结果.
type PutObjectResult struct {
	ETag      string
	VersionID string
}

// MultipartUpload 分片上传.
type MultipartUpload struct {
	Key      string
	UploadID string
	Bucket   string
}

// UploadPartResult 上传分片结果.
type UploadPartResult struct {
	PartNumber int
	ETag       string
}

// CompletedPart 已完成的分片.
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// PutOption 上传选项.
type PutOption func(*putOptions)

type putOptions struct {
	contentType        string
	contentDisposition string
	cacheControl       string
	metadata           map[string]string
	acl                string
	storageClass       string
}

// WithContentType 设置 Content-Type.
func WithContentType(contentType string) PutOption {
	return func(o *putOptions) {
		o.contentType = contentType
	}
}

// WithContentDisposition 设置 Content-Disposition.
func WithContentDisposition(disposition string) PutOption {
	return func(o *putOptions) {
		o.contentDisposition = disposition
	}
}

// WithCacheControl 设置 Cache-Control.
func WithCacheControl(cacheControl string) PutOption {
	return func(o *putOptions) {
		o.cacheControl = cacheControl
	}
}

// WithMetadata 设置元数据.
func WithMetadata(metadata map[string]string) PutOption {
	return func(o *putOptions) {
		o.metadata = metadata
	}
}

// WithACL 设置访问控制.
func WithACL(acl string) PutOption {
	return func(o *putOptions) {
		o.acl = acl
	}
}

// WithStorageClass 设置存储类型.
func WithStorageClass(storageClass string) PutOption {
	return func(o *putOptions) {
		o.storageClass = storageClass
	}
}

// ListOption 列出选项.
type ListOption func(*listOptions)

type listOptions struct {
	delimiter         string
	maxKeys           int32
	marker            string
	continuationToken string
}

// WithDelimiter 设置分隔符.
func WithDelimiter(delimiter string) ListOption {
	return func(o *listOptions) {
		o.delimiter = delimiter
	}
}

// WithMaxKeys 设置最大数量.
func WithMaxKeys(maxKeys int32) ListOption {
	return func(o *listOptions) {
		o.maxKeys = maxKeys
	}
}

// WithMarker 设置起始标记.
func WithMarker(marker string) ListOption {
	return func(o *listOptions) {
		o.marker = marker
	}
}

// WithContinuationToken 设置续传令牌.
func WithContinuationToken(token string) ListOption {
	return func(o *listOptions) {
		o.continuationToken = token
	}
}

// 常用 ACL.
const (
	// ACLPrivate 私有访问.
	ACLPrivate = "private"
	// ACLPublicRead 公开读取.
	ACLPublicRead = "public-read"
	// ACLPublicReadWrite 公开读写.
	ACLPublicReadWrite = "public-read-write"
	// ACLAuthenticatedRead 已认证用户读取.
	ACLAuthenticatedRead = "authenticated-read"
)

// 常用存储类型.
const (
	// StorageClassStandard 标准存储.
	StorageClassStandard = "STANDARD"
	// StorageClassReducedRedundancy 低冗余存储.
	StorageClassReducedRedundancy = "REDUCED_REDUNDANCY"
	// StorageClassStandardIA 标准低频访问存储.
	StorageClassStandardIA = "STANDARD_IA"
	// StorageClassOnezoneIA 单区域低频访问存储.
	StorageClassOnezoneIA = "ONEZONE_IA"
	// StorageClassGlacier 归档存储.
	StorageClassGlacier = "GLACIER"
	// StorageClassDeepArchive 深度归档存储.
	StorageClassDeepArchive = "DEEP_ARCHIVE"
)
