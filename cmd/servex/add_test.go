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
	addInfra = ""

	if err := runAddService([]string{"user"}); err != nil {
		t.Fatalf("runAddService: %v", err)
	}

	expectedFiles := []string{
		"services/user/cmd/server/main.go",
		"services/user/cmd/server/wire.go",
		"services/user/cmd/server/provider.go",
		"services/user/cmd/server/config.go",
		"services/user/internal/service/service.go",
		"services/user/internal/port/http.go",
		"services/user/internal/adapter/persistence/persistence.go",
		"services/user/configs/config.dev.yaml",
		"services/user/configs/config.prod.yaml",
		"api/user/v1/.gitkeep",
		"domain/user/.gitkeep",
		"application/user/command/.gitkeep",
		"application/user/query/.gitkeep",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q does not exist", f)
		}
	}

	// 验证 main.go 是简洁的 Wire 入口
	content, err := os.ReadFile(filepath.Join(dir, "services/user/cmd/server/main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	got := string(content)
	if !contains(got, "initApp()") {
		t.Error("main.go should call initApp()")
	}

	// grpc.go 不应存在
	grpcPath := filepath.Join(dir, "services/user/internal/port/grpc.go")
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
	addInfra = "mysql,redis"
	defer func() {
		addWithGRPC = false
		addInfra = ""
	}()

	if err := runAddService([]string{"order"}); err != nil {
		t.Fatalf("runAddService: %v", err)
	}

	// grpc.go 应存在
	grpcPath := filepath.Join(dir, "services/order/internal/port/grpc.go")
	if _, err := os.Stat(grpcPath); os.IsNotExist(err) {
		t.Error("grpc.go should exist when --with-grpc is true")
	}

	// 验证 wire.go 包含 provider 引用
	wireContent, err := os.ReadFile(filepath.Join(dir, "services/order/cmd/server/wire.go"))
	if err != nil {
		t.Fatalf("read wire.go: %v", err)
	}
	wireStr := string(wireContent)
	if !contains(wireStr, "provideMySQL") {
		t.Error("wire.go should contain provideMySQL when --infra mysql is set")
	}
	if !contains(wireStr, "provideRedis") {
		t.Error("wire.go should contain provideRedis when --infra redis is set")
	}
	if contains(wireStr, "port.NewGRPC") {
		t.Error("wire.go should not contain port.NewGRPC; provideApp assembles servers from config")
	}

	providerContent, err := os.ReadFile(filepath.Join(dir, "services/order/cmd/server/provider.go"))
	if err != nil {
		t.Fatalf("read provider.go: %v", err)
	}
	if !contains(string(providerContent), "port.NewGRPC(cfg.GRPC, log)") {
		t.Error("provider.go should assemble gRPC server from config when --with-grpc is true")
	}

	// 验证 config.dev.yaml 包含 grpc/db/redis 配置
	cfgContent, err := os.ReadFile(filepath.Join(dir, "services/order/configs/config.dev.yaml"))
	if err != nil {
		t.Fatalf("read config.dev.yaml: %v", err)
	}
	cfgStr := string(cfgContent)
	if !contains(cfgStr, "grpc:") {
		t.Error("config.dev.yaml should contain grpc section when --with-grpc is true")
	}
	if !contains(cfgStr, "database:") {
		t.Error("config.dev.yaml should contain database section when --infra mysql is set")
	}
	if !contains(cfgStr, "redis:") {
		t.Error("config.dev.yaml should contain redis section when --infra redis is set")
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
	addInfra = ""

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
