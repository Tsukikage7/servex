package minio_test

import (
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/storage/minio"
	"github.com/Tsukikage7/servex/v2/testx"
)

func TestNewClient_NilConfig(t *testing.T) {
	_, err := minio.NewClient(nil)
	if err != minio.ErrNilConfig {
		t.Errorf("期望 ErrNilConfig，得到 %v", err)
	}
}

func TestNewClient_EmptyEndpoint(t *testing.T) {
	cfg := &minio.Config{Bucket: "test"}
	_, err := minio.NewClient(cfg)
	if err != minio.ErrEmptyEndpoint {
		t.Errorf("期望 ErrEmptyEndpoint，得到 %v", err)
	}
}

func TestNewClient_EmptyBucket(t *testing.T) {
	cfg := &minio.Config{Endpoint: "localhost:9000"}
	_, err := minio.NewClient(cfg)
	if err != minio.ErrEmptyBucket {
		t.Errorf("期望 ErrEmptyBucket，得到 %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := minio.DefaultConfig()
	if cfg.Region == "" {
		t.Error("Region 不应为空")
	}
	if cfg.ConnectTimeout == 0 {
		t.Error("ConnectTimeout 不应为 0")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *minio.Config
		wantErr error
	}{
		{
			name:    "空端点",
			cfg:     &minio.Config{Bucket: "test"},
			wantErr: minio.ErrEmptyEndpoint,
		},
		{
			name:    "空桶名",
			cfg:     &minio.Config{Endpoint: "localhost:9000"},
			wantErr: minio.ErrEmptyBucket,
		},
		{
			name:    "有效配置",
			cfg:     &minio.Config{Endpoint: "localhost:9000", Bucket: "test"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err != tt.wantErr {
				t.Errorf("期望 %v，得到 %v", tt.wantErr, err)
			}
		})
	}
}

func TestConfigApplyDefaults(t *testing.T) {
	cfg := &minio.Config{Endpoint: "localhost:9000", Bucket: "test"}
	cfg.ApplyDefaults()
	if cfg.Region == "" {
		t.Error("ApplyDefaults 后 Region 不应为空")
	}
	if cfg.ConnectTimeout == 0 {
		t.Error("ApplyDefaults 后 ConnectTimeout 不应为 0")
	}
}

func TestNewClient_Success(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	if client == nil {
		t.Error("客户端不应为 nil")
	}
}

func TestNewClient_WithLogger(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg, minio.WithLogger(nil))
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	if client == nil {
		t.Error("客户端不应为 nil")
	}
}

func TestPutObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.PutObject(t.Context(), "", nil, 0, "")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestGetObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.GetObject(t.Context(), "")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestDeleteObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	err = client.DeleteObject(t.Context(), "")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestStatObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.StatObject(t.Context(), "")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestPresignGetObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.PresignGetObject(t.Context(), "", 0)
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestPresignPutObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.PresignPutObject(t.Context(), "", 0)
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestCopyObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.CopyObject(t.Context(), "", "dest")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}

	_, err = client.CopyObject(t.Context(), "src", "")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestFGetObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	err = client.FGetObject(t.Context(), "", "/tmp/test")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestFPutObject_EmptyKey(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	_, err = client.FPutObject(t.Context(), "", "/tmp/test", "")
	if err != minio.ErrEmptyKey {
		t.Errorf("期望 ErrEmptyKey，得到 %v", err)
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := minio.DefaultConfig()
	if cfg.Region != "us-east-1" {
		t.Errorf("期望 Region=us-east-1，得到 %s", cfg.Region)
	}
	if cfg.UseSSL != false {
		t.Error("期望 UseSSL=false")
	}
	if cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("期望 ConnectTimeout=10s，得到 %v", cfg.ConnectTimeout)
	}
}

func TestConfigApplyDefaults_NoOverwrite(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:       "localhost:9000",
		Bucket:         "test",
		Region:         "eu-west-1",
		ConnectTimeout: 30 * time.Second,
	}
	cfg.ApplyDefaults()
	if cfg.Region != "eu-west-1" {
		t.Errorf("不应覆盖已设置的 Region，得到 %s", cfg.Region)
	}
	if cfg.ConnectTimeout != 30*time.Second {
		t.Errorf("不应覆盖已设置的 ConnectTimeout，得到 %v", cfg.ConnectTimeout)
	}
}

func TestNewClient_WithRealLogger(t *testing.T) {
	log := testx.NopLogger()
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
	}
	client, err := minio.NewClient(cfg, minio.WithLogger(log))
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	if client == nil {
		t.Error("客户端不应为 nil")
	}
}

func TestNewClient_WithSSL(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
		UseSSL:    true,
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	if client == nil {
		t.Error("客户端不应为 nil")
	}
}

func TestNewClient_WithRegion(t *testing.T) {
	cfg := &minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test",
		Region:    "ap-northeast-1",
	}
	client, err := minio.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	if client == nil {
		t.Error("客户端不应为 nil")
	}
}

// 确保导入不报错.
var (
	_ logger.Logger
	_ time.Duration
)
