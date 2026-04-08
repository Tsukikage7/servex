package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAddService 测试在 monorepo 中添加微服务.
func TestAddService(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// 构建 monorepo 结构
	os.MkdirAll("services", 0o755)
	os.WriteFile("go.mod", []byte("module github.com/example/monorepo\n\ngo 1.22\n"), 0o644)

	// 设置 flags
	addWithGRPC = false
	addWithDB = false
	addWithRedis = false
	addWithWire = false

	if err := runAddService([]string{"user"}); err != nil {
		t.Fatalf("runAddService: %v", err)
	}

	expectedFiles := []string{
		"services/user-service/cmd/server/main.go",
		"services/user-service/internal/server/http.go",
		"services/user-service/internal/data/data.go",
		"services/user-service/configs/config.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q does not exist", f)
		}
	}

	// 验证 main.go 引用正确的 module 路径
	content, err := os.ReadFile(filepath.Join(dir, "services/user-service/cmd/server/main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	got := string(content)
	if !contains(got, "github.com/example/monorepo/services/user-service/internal/server") {
		t.Error("main.go should reference monorepo service import path")
	}
	if !contains(got, `app.WithName("user-service")`) {
		t.Error("main.go should contain service name")
	}

	// grpc.go 不应存在
	grpcPath := filepath.Join(dir, "services/user-service/internal/server/grpc.go")
	if _, err := os.Stat(grpcPath); !os.IsNotExist(err) {
		t.Error("grpc.go should not exist when --with-grpc is false")
	}
}

// TestAddServiceWithGRPC 测试添加包含 gRPC 的微服务.
func TestAddServiceWithGRPC(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	os.MkdirAll("services", 0o755)
	os.WriteFile("go.mod", []byte("module github.com/example/monorepo\n\ngo 1.22\n"), 0o644)

	addWithGRPC = true
	addWithDB = true
	addWithRedis = true
	addWithWire = false
	defer func() {
		addWithGRPC = false
		addWithDB = false
		addWithRedis = false
	}()

	if err := runAddService([]string{"order"}); err != nil {
		t.Fatalf("runAddService: %v", err)
	}

	// grpc.go 应存在
	grpcPath := filepath.Join(dir, "services/order-service/internal/server/grpc.go")
	if _, err := os.Stat(grpcPath); os.IsNotExist(err) {
		t.Error("grpc.go should exist when --with-grpc is true")
	}

	// 验证 main.go 包含 DB 和 Redis 初始化
	content, err := os.ReadFile(filepath.Join(dir, "services/order-service/cmd/server/main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	got := string(content)
	if !contains(got, "rdbms.NewDatabase") {
		t.Error("main.go should contain database setup when --with-db is true")
	}
	if !contains(got, "srvredis.NewClient") {
		t.Error("main.go should contain redis setup when --with-redis is true")
	}
	if !contains(got, "server.NewGRPC") {
		t.Error("main.go should contain gRPC server setup when --with-grpc is true")
	}

	// 验证 config.yaml 包含 grpc/db/redis 配置
	cfgContent, err := os.ReadFile(filepath.Join(dir, "services/order-service/configs/config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	cfgStr := string(cfgContent)
	if !contains(cfgStr, "grpc:") {
		t.Error("config.yaml should contain grpc section when --with-grpc is true")
	}
	if !contains(cfgStr, "database:") {
		t.Error("config.yaml should contain database section when --with-db is true")
	}
	if !contains(cfgStr, "redis:") {
		t.Error("config.yaml should contain redis section when --with-redis is true")
	}
}

// TestAddServiceNotMonorepo 测试在非 monorepo 目录下报错.
func TestAddServiceNotMonorepo(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// 不创建 services/ 和 go.mod，模拟非 monorepo
	err := runAddService([]string{"user"})
	if err == nil {
		t.Fatal("expected error when not in monorepo, got nil")
	}
	if !contains(err.Error(), "monorepo") {
		t.Errorf("error should mention monorepo, got: %v", err)
	}
}

// TestAddServiceDuplicate 测试重复添加服务报错.
func TestAddServiceDuplicate(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	os.MkdirAll("services", 0o755)
	os.WriteFile("go.mod", []byte("module github.com/example/monorepo\n\ngo 1.22\n"), 0o644)

	addWithGRPC = false
	addWithDB = false
	addWithRedis = false
	addWithWire = false

	// 第一次添加应成功
	if err := runAddService([]string{"user"}); err != nil {
		t.Fatalf("first runAddService: %v", err)
	}

	// 第二次添加同名服务应失败
	err := runAddService([]string{"user"})
	if err == nil {
		t.Fatal("expected error when service already exists, got nil")
	}
	if !contains(err.Error(), "已存在") {
		t.Errorf("error should mention service already exists, got: %v", err)
	}
}
