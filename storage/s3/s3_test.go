package s3_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/s3"
	"github.com/Tsukikage7/servex/v2/testx"
)

// 默认指向本地 MinIO 实例.
var (
	s3Endpoint  = "http://localhost:9000"
	s3AccessKey = "minioadmin"
	s3SecretKey = "minioadmin"
	s3Bucket    = "servex-test"

	// s3Available 由 TestMain 探测，避免每个测试重复连接.
	s3Available bool
)

func TestMain(m *testing.M) {
	if ep := os.Getenv("S3_ENDPOINT"); ep != "" {
		s3Endpoint = ep
	}
	if ak := os.Getenv("S3_ACCESS_KEY"); ak != "" {
		s3AccessKey = ak
	}
	if sk := os.Getenv("S3_SECRET_KEY"); sk != "" {
		s3SecretKey = sk
	}
	if bk := os.Getenv("S3_BUCKET"); bk != "" {
		s3Bucket = bk
	}

	// 统一探测 S3/MinIO 连通性一次即可
	s3Available = probeS3()

	os.Exit(m.Run())
}

// probeS3 探测 S3/MinIO 是否可用.
func probeS3() bool {
	cfg := &s3.Config{
		Endpoint:       s3Endpoint,
		AccessKey:      s3AccessKey,
		SecretKey:      s3SecretKey,
		Bucket:         s3Bucket,
		UseSSL:         false,
		UsePathStyle:   true,
		ConnectTimeout: 2 * time.Second,
		RequestTimeout: 2 * time.Second,
		MaxRetries:     0,
	}
	client, err := s3.NewClient(cfg, testx.NopLogger())
	if err != nil {
		return false
	}
	defer client.Close()

	// BucketExists 将所有错误视为 false，须用 ListBuckets 探测连通性
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = client.ListBuckets(ctx)
	return err == nil
}

func skipIfNoS3(t *testing.T) {
	t.Helper()
	if !s3Available {
		t.Skip("S3/MinIO 不可用，跳过集成测试")
	}
}

func newTestClient(t *testing.T) s3.Client {
	t.Helper()

	cfg := &s3.Config{
		Endpoint:       s3Endpoint,
		AccessKey:      s3AccessKey,
		SecretKey:      s3SecretKey,
		Bucket:         s3Bucket,
		UseSSL:         false,
		UsePathStyle:   true,
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 10 * time.Second,
	}

	client, err := s3.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Fatalf("创建 S3 客户端失败: %v", err)
	}
	return client
}

// ---- 单元测试不需要服务----

func TestNewClient_NilConfig(t *testing.T) {
	_, err := s3.NewClient(nil, testx.NopLogger())
	if err != s3.ErrNilConfig {
		t.Errorf("期望 ErrNilConfig，得到 %v", err)
	}
}

func TestNewClient_NilLogger(t *testing.T) {
	cfg := &s3.Config{Endpoint: "http://localhost:9000", Bucket: "test"}
	_, err := s3.NewClient(cfg, nil)
	if err != s3.ErrNilLogger {
		t.Errorf("期望 ErrNilLogger，得到 %v", err)
	}
}

func TestNewClient_EmptyEndpoint(t *testing.T) {
	cfg := &s3.Config{Bucket: "test"}
	_, err := s3.NewClient(cfg, testx.NopLogger())
	if err != s3.ErrEmptyEndpoint {
		t.Errorf("期望 ErrEmptyEndpoint，得到 %v", err)
	}
}

func TestNewClient_EmptyBucket(t *testing.T) {
	cfg := &s3.Config{Endpoint: "http://localhost:9000"}
	_, err := s3.NewClient(cfg, testx.NopLogger())
	if err != s3.ErrEmptyBucket {
		t.Errorf("期望 ErrEmptyBucket，得到 %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := s3.DefaultConfig()
	if cfg.Region == "" {
		t.Error("Region 不应为空")
	}
	if cfg.PartSize == 0 {
		t.Error("PartSize 不应为 0")
	}
}

// ---- 集成测试，需要 MinIO 实例 ----

func TestBucketExists(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	exists, err := client.BucketExists(ctx, s3Bucket)
	if err != nil {
		t.Fatalf("BucketExists 失败: %v", err)
	}
	if !exists {
		if err = client.CreateBucket(ctx, s3Bucket); err != nil {
			t.Fatalf("CreateBucket 失败: %v", err)
		}
	}
}

func TestPutAndGetObject(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	const key = "test/hello.txt"
	const content = "Hello, servex S3!"

	if exists, _ := client.BucketExists(ctx, s3Bucket); !exists {
		_ = client.CreateBucket(ctx, s3Bucket)
	}

	// 上传
	reader := strings.NewReader(content)
	res, err := client.PutObject(ctx, key, reader, int64(len(content)),
		s3.WithContentType("text/plain"))
	if err != nil {
		t.Fatalf("PutObject 失败: %v", err)
	}
	if res.ETag == "" {
		t.Error("ETag 不应为空")
	}

	// 下载并验证
	obj, err := client.GetObject(ctx, key)
	if err != nil {
		t.Fatalf("GetObject 失败: %v", err)
	}
	defer obj.Body.Close()

	data, err := io.ReadAll(obj.Body)
	if err != nil {
		t.Fatalf("读取对象内容失败: %v", err)
	}
	if string(data) != content {
		t.Errorf("期望 %q，得到 %q", content, string(data))
	}

	_ = client.DeleteObject(ctx, key)
}

func TestDownload(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	const key = "test/download.txt"
	const content = "Download test content"

	if exists, _ := client.BucketExists(ctx, s3Bucket); !exists {
		_ = client.CreateBucket(ctx, s3Bucket)
	}

	if _, err := client.PutObject(ctx, key, strings.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("PutObject 失败: %v", err)
	}
	defer client.DeleteObject(ctx, key) //nolint

	var buf bytes.Buffer
	n, err := client.Download(ctx, key, &buf)
	if err != nil {
		t.Fatalf("Download 失败: %v", err)
	}
	if n != int64(len(content)) {
		t.Errorf("期望下载 %d 字节，得到 %d", len(content), n)
	}
	if buf.String() != content {
		t.Errorf("期望 %q，得到 %q", content, buf.String())
	}
}

func TestHeadObject(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	const key = "test/head.txt"
	const content = "Head test"

	if exists, _ := client.BucketExists(ctx, s3Bucket); !exists {
		_ = client.CreateBucket(ctx, s3Bucket)
	}

	if _, err := client.PutObject(ctx, key, strings.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("PutObject 失败: %v", err)
	}
	defer client.DeleteObject(ctx, key) //nolint

	info, err := client.HeadObject(ctx, key)
	if err != nil {
		t.Fatalf("HeadObject 失败: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("期望 Size=%d，得到 %d", len(content), info.Size)
	}
}

func TestObjectExists(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	const key = "test/exists.txt"

	if exists, _ := client.BucketExists(ctx, s3Bucket); !exists {
		_ = client.CreateBucket(ctx, s3Bucket)
	}

	ok, err := client.ObjectExists(ctx, key)
	if err != nil {
		t.Fatalf("ObjectExists 失败: %v", err)
	}
	if ok {
		t.Error("对象不应存在")
	}

	if _, err = client.PutObject(ctx, key, strings.NewReader("x"), 1); err != nil {
		t.Fatalf("PutObject 失败: %v", err)
	}
	defer client.DeleteObject(ctx, key) //nolint

	ok, err = client.ObjectExists(ctx, key)
	if err != nil {
		t.Fatalf("ObjectExists 失败: %v", err)
	}
	if !ok {
		t.Error("对象应存在")
	}
}

func TestPresignGetObject(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()
	const key = "test/presign.txt"

	if exists, _ := client.BucketExists(ctx, s3Bucket); !exists {
		_ = client.CreateBucket(ctx, s3Bucket)
	}

	if _, err := client.PutObject(ctx, key, strings.NewReader("presign"), 7); err != nil {
		t.Fatalf("PutObject 失败: %v", err)
	}
	defer client.DeleteObject(ctx, key) //nolint

	url, err := client.PresignGetObject(ctx, key, 1*time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObject 失败: %v", err)
	}
	if url == "" {
		t.Error("预签名 URL 不应为空")
	}
	if !strings.HasPrefix(url, "http") {
		t.Errorf("预签名 URL 格式错误: %s", url)
	}
}

func TestDeleteObjects(t *testing.T) {
	skipIfNoS3(t)
	client := newTestClient(t)
	defer client.Close()

	ctx := t.Context()

	if exists, _ := client.BucketExists(ctx, s3Bucket); !exists {
		_ = client.CreateBucket(ctx, s3Bucket)
	}

	keys := []string{"test/del1.txt", "test/del2.txt", "test/del3.txt"}
	for _, k := range keys {
		if _, err := client.PutObject(ctx, k, strings.NewReader("del"), 3); err != nil {
			t.Fatalf("PutObject %s 失败: %v", k, err)
		}
	}

	if err := client.DeleteObjects(ctx, keys); err != nil {
		t.Fatalf("DeleteObjects 失败: %v", err)
	}

	for _, k := range keys {
		ok, _ := client.ObjectExists(ctx, k)
		if ok {
			t.Errorf("对象 %s 应已删除", k)
		}
	}
}
