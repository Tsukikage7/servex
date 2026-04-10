package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/Tsukikage7/servex/observability/logger"
)

// s3Client S3 客户端实现.
type s3Client struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	config    *Config
	log       logger.Logger
}

// NewClient 创建 S3 客户端.
func NewClient(cfg *Config, log logger.Logger) (Client, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// 创建 AWS 配置
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.Endpoint,
			SigningRegion:     cfg.Region,
			HostnameImmutable: cfg.UsePathStyle,
		}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithRetryMaxAttempts(cfg.MaxRetries),
	)
	if err != nil {
		return nil, err
	}

	// 创建 S3 客户端
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	log.Info("s3 client created", "endpoint", cfg.Endpoint, "bucket", cfg.Bucket)

	return &s3Client{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
		config:    cfg,
		log:       log,
	}, nil
}

// MustNewClient 创建 S3 客户端，失败时 panic.
func MustNewClient(cfg *Config, log logger.Logger) Client {
	client, err := NewClient(cfg, log)
	if err != nil {
		panic(err)
	}
	return client
}

// Bucket 操作

func (c *s3Client) CreateBucket(ctx context.Context, bucket string) error {
	_, err := c.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (c *s3Client) DeleteBucket(ctx context.Context, bucket string) error {
	_, err := c.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (c *s3Client) BucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// 404/NotFound 表示桶不存在，其他错误向上传播
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *s3Client) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	result, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	buckets := make([]BucketInfo, len(result.Buckets))
	for i, b := range result.Buckets {
		buckets[i] = BucketInfo{
			Name:         aws.ToString(b.Name),
			CreationDate: aws.ToTime(b.CreationDate),
		}
	}
	return buckets, nil
}

// Object 操作

func (c *s3Client) PutObject(ctx context.Context, key string, reader io.Reader, size int64, opts ...PutOption) (*PutObjectResult, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	o := &putOptions{}
	for _, opt := range opts {
		opt(o)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
	}

	if o.contentType != "" {
		input.ContentType = aws.String(o.contentType)
	}
	if o.contentDisposition != "" {
		input.ContentDisposition = aws.String(o.contentDisposition)
	}
	if o.cacheControl != "" {
		input.CacheControl = aws.String(o.cacheControl)
	}
	if o.metadata != nil {
		input.Metadata = o.metadata
	}
	if o.storageClass != "" {
		input.StorageClass = types.StorageClass(o.storageClass)
	}

	result, err := c.client.PutObject(ctx, input)
	if err != nil {
		return nil, err
	}

	return &PutObjectResult{
		ETag:      aws.ToString(result.ETag),
		VersionID: aws.ToString(result.VersionId),
	}, nil
}

func (c *s3Client) GetObject(ctx context.Context, key string) (*Object, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return &Object{
		Key:           key,
		Body:          result.Body,
		ContentType:   aws.ToString(result.ContentType),
		ContentLength: aws.ToInt64(result.ContentLength),
		ETag:          aws.ToString(result.ETag),
		LastModified:  aws.ToTime(result.LastModified),
		Metadata:      result.Metadata,
	}, nil
}

func (c *s3Client) DeleteObject(ctx context.Context, key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *s3Client) DeleteObjects(ctx context.Context, keys []string) error {
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{
			Key: aws.String(key),
		}
	}

	_, err := c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(c.bucket),
		Delete: &types.Delete{
			Objects: objects,
		},
	})
	return err
}

func (c *s3Client) CopyObject(ctx context.Context, srcKey, destKey string) error {
	_, err := c.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(c.bucket),
		CopySource: aws.String(c.bucket + "/" + srcKey),
		Key:        aws.String(destKey),
	})
	return err
}

func (c *s3Client) HeadObject(ctx context.Context, key string) (*ObjectInfo, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	result, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		ETag:         aws.ToString(result.ETag),
		ContentType:  aws.ToString(result.ContentType),
		LastModified: aws.ToTime(result.LastModified),
		StorageClass: string(result.StorageClass),
		Metadata:     result.Metadata,
	}, nil
}

func (c *s3Client) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := c.HeadObject(ctx, key)
	if err != nil {
		// 404/NotFound 表示对象不存在，其他错误向上传播
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *s3Client) ListObjects(ctx context.Context, prefix string, opts ...ListOption) (*ListObjectsResult, error) {
	o := &listOptions{
		maxKeys: 1000,
	}
	for _, opt := range opts {
		opt(o)
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(o.maxKeys),
	}

	if o.delimiter != "" {
		input.Delimiter = aws.String(o.delimiter)
	}
	if o.continuationToken != "" {
		input.ContinuationToken = aws.String(o.continuationToken)
	}

	result, err := c.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}

	objects := make([]ObjectInfo, len(result.Contents))
	for i, obj := range result.Contents {
		objects[i] = ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			ETag:         aws.ToString(obj.ETag),
			LastModified: aws.ToTime(obj.LastModified),
			StorageClass: string(obj.StorageClass),
		}
	}

	prefixes := make([]string, len(result.CommonPrefixes))
	for i, p := range result.CommonPrefixes {
		prefixes[i] = aws.ToString(p.Prefix)
	}

	return &ListObjectsResult{
		Objects:               objects,
		Prefixes:              prefixes,
		IsTruncated:           aws.ToBool(result.IsTruncated),
		NextContinuationToken: aws.ToString(result.NextContinuationToken),
	}, nil
}

// 分片上传

func (c *s3Client) CreateMultipartUpload(ctx context.Context, key string, opts ...PutOption) (*MultipartUpload, error) {
	o := &putOptions{}
	for _, opt := range opts {
		opt(o)
	}

	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	if o.contentType != "" {
		input.ContentType = aws.String(o.contentType)
	}
	if o.metadata != nil {
		input.Metadata = o.metadata
	}
	if o.storageClass != "" {
		input.StorageClass = types.StorageClass(o.storageClass)
	}

	result, err := c.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return nil, err
	}

	return &MultipartUpload{
		Key:      key,
		UploadID: aws.ToString(result.UploadId),
		Bucket:   c.bucket,
	}, nil
}

func (c *s3Client) UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int, reader io.Reader, size int64) (*UploadPartResult, error) {
	result, err := c.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:        aws.String(upload.Bucket),
		Key:           aws.String(upload.Key),
		UploadId:      aws.String(upload.UploadID),
		PartNumber:    aws.Int32(int32(partNumber)),
		Body:          reader,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return nil, err
	}

	return &UploadPartResult{
		PartNumber: partNumber,
		ETag:       aws.ToString(result.ETag),
	}, nil
}

func (c *s3Client) CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload, parts []CompletedPart) (*PutObjectResult, error) {
	completedParts := make([]types.CompletedPart, len(parts))
	for i, part := range parts {
		completedParts[i] = types.CompletedPart{
			PartNumber: aws.Int32(int32(part.PartNumber)),
			ETag:       aws.String(part.ETag),
		}
	}

	result, err := c.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(upload.Bucket),
		Key:      aws.String(upload.Key),
		UploadId: aws.String(upload.UploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return nil, err
	}

	return &PutObjectResult{
		ETag:      aws.ToString(result.ETag),
		VersionID: aws.ToString(result.VersionId),
	}, nil
}

func (c *s3Client) AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error {
	_, err := c.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(upload.Bucket),
		Key:      aws.String(upload.Key),
		UploadId: aws.String(upload.UploadID),
	})
	return err
}

// 预签名 URL

func (c *s3Client) PresignGetObject(ctx context.Context, key string, expires time.Duration) (string, error) {
	result, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (c *s3Client) PresignPutObject(ctx context.Context, key string, expires time.Duration) (string, error) {
	result, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// 工具方法

func (c *s3Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, opts ...PutOption) (*PutObjectResult, error) {
	// 小于 5MB 使用普通上传
	if size < c.config.PartSize {
		return c.PutObject(ctx, key, reader, size, opts...)
	}

	// 大文件使用分片上传
	upload, err := c.CreateMultipartUpload(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	var parts []CompletedPart
	partNumber := 1
	remaining := size

	for remaining > 0 {
		partSize := c.config.PartSize
		if remaining < partSize {
			partSize = remaining
		}

		// 读取分片
		partReader := io.LimitReader(reader, partSize)
		result, err := c.UploadPart(ctx, upload, partNumber, partReader, partSize)
		if err != nil {
			_ = c.AbortMultipartUpload(ctx, upload)
			return nil, err
		}

		parts = append(parts, CompletedPart{
			PartNumber: result.PartNumber,
			ETag:       result.ETag,
		})

		remaining -= partSize
		partNumber++
	}

	return c.CompleteMultipartUpload(ctx, upload, parts)
}

func (c *s3Client) Download(ctx context.Context, key string, writer io.Writer) (int64, error) {
	obj, err := c.GetObject(ctx, key)
	if err != nil {
		return 0, err
	}
	defer obj.Body.Close()

	return io.Copy(writer, obj.Body)
}

func (c *s3Client) UseBucket(bucket string) Client {
	return &s3Client{
		client:    c.client,
		presigner: c.presigner,
		bucket:    bucket,
		config:    c.config,
		log:       c.log,
	}
}

func (c *s3Client) Close() error {
	c.log.Info("s3 client closed")
	return nil
}

// isNotFound 判断 S3 错误是否为 404 Not Found.
func isNotFound(err error) bool {
	// 检查 AWS SDK 特定的 NotFound 错误类型
	var nsk *types.NotFound
	if errors.As(err, &nsk) {
		return true
	}
	var nsb *types.NoSuchBucket
	if errors.As(err, &nsb) {
		return true
	}
	var nso *types.NoSuchKey
	if errors.As(err, &nso) {
		return true
	}
	// 检查 HTTP 状态码 404
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode() == http.StatusNotFound
	}
	return false
}
