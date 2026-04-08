package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunProtoAdd 测试 proto add 生成 .proto 文件.
func TestRunProtoAdd(t *testing.T) {
	dir := t.TempDir()

	err := runProtoAdd([]string{"order", "--module", "github.com/example/myapp", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd: %v", err)
	}

	protoPath := filepath.Join(dir, "api/order/v1/order.proto")
	content, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("read proto file: %v", err)
	}

	text := string(content)

	checks := []struct {
		desc   string
		substr string
	}{
		{"syntax", `syntax = "proto3";`},
		{"package", `package order.v1;`},
		{"go_package", `option go_package = "github.com/example/myapp/api/order/v1;orderv1";`},
		{"service name", `service OrderService {`},
		{"CreateOrder rpc", `rpc CreateOrder`},
		{"GetOrder rpc", `rpc GetOrder`},
		{"ListOrder rpc", `rpc ListOrder`},
		{"UpdateOrder rpc", `rpc UpdateOrder`},
		{"DeleteOrder rpc", `rpc DeleteOrder`},
		{"http post", `post: "/v1/order"`},
		{"http get by id", `get: "/v1/order/{id}"`},
		{"http delete", `delete: "/v1/order/{id}"`},
		{"annotations import", `google/api/annotations.proto`},
	}
	for _, c := range checks {
		if !contains(text, c.substr) {
			t.Errorf("proto file missing %s: expected %q", c.desc, c.substr)
		}
	}
}

// TestRunProtoAddSnakeCase 测试 snake_case 名称的 proto add.
func TestRunProtoAddSnakeCase(t *testing.T) {
	dir := t.TempDir()

	err := runProtoAdd([]string{"user_profile", "--module", "github.com/example/app", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd: %v", err)
	}

	protoPath := filepath.Join(dir, "api/user_profile/v1/user_profile.proto")
	content, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("read proto file: %v", err)
	}

	text := string(content)
	if !contains(text, "service UserProfileService") {
		t.Error("proto file should contain PascalCase service name")
	}
	if !contains(text, "rpc CreateUserProfile") {
		t.Error("proto file should contain CreateUserProfile rpc")
	}
}

// TestRunProtoAddNoArgs 测试无参数调用 proto add.
func TestRunProtoAddNoArgs(t *testing.T) {
	err := runProtoAdd(nil)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

// TestParseProtoFile 测试 proto 文件解析.
func TestParseProtoFile(t *testing.T) {
	dir := t.TempDir()

	protoContent := `syntax = "proto3";
package order.v1;
service OrderService {
  rpc CreateOrder (CreateOrderRequest) returns (CreateOrderReply) {}
  rpc GetOrder (GetOrderRequest) returns (GetOrderReply) {}
  rpc ListOrder (ListOrderRequest) returns (ListOrderReply) {}
}
`
	protoPath := filepath.Join(dir, "order.proto")
	if err := os.WriteFile(protoPath, []byte(protoContent), 0o644); err != nil {
		t.Fatalf("write proto file: %v", err)
	}

	serviceName, rpcs, err := parseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("parseProtoFile: %v", err)
	}

	if serviceName != "Order" {
		t.Errorf("serviceName = %q, want %q", serviceName, "Order")
	}
	if len(rpcs) != 3 {
		t.Fatalf("rpcs count = %d, want 3", len(rpcs))
	}

	if rpcs[0].MethodName != "CreateOrder" {
		t.Errorf("rpcs[0].MethodName = %q, want %q", rpcs[0].MethodName, "CreateOrder")
	}
	if rpcs[0].RequestType != "CreateOrderRequest" {
		t.Errorf("rpcs[0].RequestType = %q, want %q", rpcs[0].RequestType, "CreateOrderRequest")
	}
	if rpcs[0].ReplyType != "CreateOrderReply" {
		t.Errorf("rpcs[0].ReplyType = %q, want %q", rpcs[0].ReplyType, "CreateOrderReply")
	}
}

// TestRunProtoServer 测试 proto server 生成 Go 服务桩代码.
func TestRunProtoServer(t *testing.T) {
	dir := t.TempDir()

	// 先生成 proto 文件
	err := runProtoAdd([]string{"order", "--module", "github.com/example/myapp", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd: %v", err)
	}

	protoPath := filepath.Join(dir, "api/order/v1/order.proto")
	targetDir := filepath.Join(dir, "internal/service")

	err = runProtoServer([]string{protoPath, "--target", targetDir})
	if err != nil {
		t.Fatalf("runProtoServer: %v", err)
	}

	outPath := filepath.Join(targetDir, "order.go")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read server stub: %v", err)
	}

	text := string(content)

	checks := []struct {
		desc   string
		substr string
	}{
		{"package", "package service"},
		{"struct", "type OrderService struct"},
		{"unimplemented", "Unimplemented"},
		{"CreateOrder method", "func (s *OrderService) CreateOrder(ctx context.Context"},
		{"GetOrder method", "func (s *OrderService) GetOrder(ctx context.Context"},
		{"ListOrder method", "func (s *OrderService) ListOrder(ctx context.Context"},
		{"UpdateOrder method", "func (s *OrderService) UpdateOrder(ctx context.Context"},
		{"DeleteOrder method", "func (s *OrderService) DeleteOrder(ctx context.Context"},
		{"TODO comment", "// TODO: implement"},
		{"context import", `"context"`},
		{"pb import", `pb "github.com/example/myapp/api/order/v1"`},
		{"constructor", "NewOrderService"},
	}
	for _, c := range checks {
		if !contains(text, c.substr) {
			t.Errorf("server stub missing %s: expected %q", c.desc, c.substr)
		}
	}
}

// TestRunProtoServerNoArgs 测试无参数调用 proto server.
func TestRunProtoServerNoArgs(t *testing.T) {
	err := runProtoServer(nil)
	if err == nil {
		t.Error("expected error for missing proto file")
	}
}

// TestHasHTTPAnnotations 测试 HTTP 注解检测.
func TestHasHTTPAnnotations(t *testing.T) {
	dir := t.TempDir()

	// 包含注解
	withAnnotations := filepath.Join(dir, "with.proto")
	if err := os.WriteFile(withAnnotations, []byte(`import "google/api/annotations.proto";`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasHTTPAnnotations(withAnnotations) {
		t.Error("expected true for proto with annotations")
	}

	// 不包含注解
	withoutAnnotations := filepath.Join(dir, "without.proto")
	if err := os.WriteFile(withoutAnnotations, []byte(`syntax = "proto3";`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasHTTPAnnotations(withoutAnnotations) {
		t.Error("expected false for proto without annotations")
	}

	// 文件不存在
	if hasHTTPAnnotations(filepath.Join(dir, "nonexistent.proto")) {
		t.Error("expected false for nonexistent file")
	}
}

// TestDetectModuleFromProto 测试从 proto 文件提取 module 路径.
func TestDetectModuleFromProto(t *testing.T) {
	dir := t.TempDir()

	protoContent := `syntax = "proto3";
package order.v1;
option go_package = "github.com/example/myapp/api/order/v1;orderv1";
`
	protoPath := filepath.Join(dir, "order.proto")
	if err := os.WriteFile(protoPath, []byte(protoContent), 0o644); err != nil {
		t.Fatal(err)
	}

	mod, err := detectModuleFromProto(protoPath)
	if err != nil {
		t.Fatalf("detectModuleFromProto: %v", err)
	}
	if mod != "github.com/example/myapp" {
		t.Errorf("module = %q, want %q", mod, "github.com/example/myapp")
	}
}

// TestRunProtoNoArgs 测试无参数调用 proto.
func TestRunProtoNoArgs(t *testing.T) {
	err := runProto(nil)
	if err == nil {
		t.Error("expected error for missing subcommand")
	}
}

// TestRunProtoUnknown 测试未知子命令.
func TestRunProtoUnknown(t *testing.T) {
	err := runProto([]string{"unknown"})
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// TestRunProtoHelp 测试 help 子命令.
func TestRunProtoHelp(t *testing.T) {
	err := runProto([]string{"help"})
	if err != nil {
		t.Errorf("unexpected error for help: %v", err)
	}
}

// TestProtoServerMonorepo 测试 monorepo 模式下 proto server 输出路径.
func TestProtoServerMonorepo(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// 创建 services/ 目录模拟 monorepo
	if err := os.Mkdir(filepath.Join(dir, "services"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 先生成 proto 文件
	err := runProtoAdd([]string{"order", "--module", "github.com/example/monorepo", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd: %v", err)
	}

	protoPath := filepath.Join(dir, "api/order/v1/order.proto")

	// 使用 --service 标志触发 monorepo 模式
	err = runProtoServer([]string{protoPath, "--service", "order"})
	if err != nil {
		t.Fatalf("runProtoServer with --service: %v", err)
	}

	// 验证输出到 services/order-service/internal/service/order.go
	outPath := filepath.Join("services", "order-service", "internal", "service", "order.go")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("expected server stub at %q in monorepo mode", outPath)
	}
}

// TestBuildProtoServerArgs 测试 proto server 参数构建.
func TestBuildProtoServerArgs(t *testing.T) {
	// 默认参数: 仅 proto 文件
	args := buildProtoServerArgs("test.proto", "internal/service", "")
	if len(args) != 1 || args[0] != "test.proto" {
		t.Errorf("default args = %v, want [test.proto]", args)
	}

	// 自定义 target
	args = buildProtoServerArgs("test.proto", "custom/path", "")
	if len(args) != 3 {
		t.Fatalf("args with target: len = %d, want 3", len(args))
	}
	if args[1] != "-target" || args[2] != "custom/path" {
		t.Errorf("args with target = %v, want [test.proto -target custom/path]", args)
	}

	// 带 service 标志
	args = buildProtoServerArgs("test.proto", "internal/service", "order")
	if len(args) != 3 {
		t.Fatalf("args with service: len = %d, want 3", len(args))
	}
	if args[1] != "-service" || args[2] != "order" {
		t.Errorf("args with service = %v, want [test.proto -service order]", args)
	}
}
