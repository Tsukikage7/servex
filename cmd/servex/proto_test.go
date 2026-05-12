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
		t.Fatalf("runProtoAdd 失败: %v", err)
	}

	protoPath := filepath.Join(dir, "api/order/v1/order.proto")
	content, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("读取 proto 文件失败: %v", err)
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
			t.Errorf("proto 文件缺少 %s: 期望包含 %q", c.desc, c.substr)
		}
	}
}

// TestRunProtoAddSnakeCase 测试 snake_case 名称的 proto add.
func TestRunProtoAddSnakeCase(t *testing.T) {
	dir := t.TempDir()

	err := runProtoAdd([]string{"user_profile", "--module", "github.com/example/app", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd 失败: %v", err)
	}

	protoPath := filepath.Join(dir, "api/user_profile/v1/user_profile.proto")
	content, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("读取 proto 文件失败: %v", err)
	}

	text := string(content)
	if !contains(text, "service UserProfileService") {
		t.Error("proto 文件应包含 PascalCase 服务名")
	}
	if !contains(text, "rpc CreateUserProfile") {
		t.Error("proto 文件应包含 CreateUserProfile rpc")
	}
}

// TestRunProtoAddNoArgs 测试无参数调用 proto add.
func TestRunProtoAddNoArgs(t *testing.T) {
	err := runProtoAdd(nil)
	if err == nil {
		t.Error("缺少名称参数时应返回错误")
	}
}

// TestRunProtoAddGeneratesBufConfig 测试 proto add 自动生成 buf 配置文件.
func TestRunProtoAddGeneratesBufConfig(t *testing.T) {
	dir := t.TempDir()

	err := runProtoAdd([]string{"product", "--module", "github.com/example/shop", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd 失败: %v", err)
	}

	// 检查 buf.yaml 已生成
	bufYaml := filepath.Join(dir, "api", "buf.yaml")
	content, err := os.ReadFile(bufYaml)
	if err != nil {
		t.Fatalf("buf.yaml 未生成: %v", err)
	}
	text := string(content)
	if !contains(text, "version: v2") {
		t.Error("buf.yaml 缺少 version: v2")
	}
	if !contains(text, "path: ../third_party") {
		t.Error("buf.yaml 缺少本地 third_party 依赖")
	}

	// 检查 buf.gen.yaml 已生成
	bufGenYaml := filepath.Join(dir, "api", "buf.gen.yaml")
	content, err = os.ReadFile(bufGenYaml)
	if err != nil {
		t.Fatalf("buf.gen.yaml 未生成: %v", err)
	}
	text = string(content)
	if !contains(text, "buf.build/protocolbuffers/go") {
		t.Error("buf.gen.yaml 缺少 protoc-gen-go 插件")
	}
	if !contains(text, "buf.build/grpc/go") {
		t.Error("buf.gen.yaml 缺少 grpc 插件")
	}
}

// TestRunProtoAddSkipsExistingBufConfig 测试 proto add 不覆盖已有 buf 配置.
func TestRunProtoAddSkipsExistingBufConfig(t *testing.T) {
	dir := t.TempDir()

	// 先创建自定义 buf.yaml
	apiDir := filepath.Join(dir, "api")
	os.MkdirAll(apiDir, 0o755)
	customContent := "# custom buf.yaml\nversion: v2\n"
	os.WriteFile(filepath.Join(apiDir, "buf.yaml"), []byte(customContent), 0o644)

	err := runProtoAdd([]string{"order", "--module", "github.com/example/myapp", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd 失败: %v", err)
	}

	// buf.yaml 应保持不变
	content, _ := os.ReadFile(filepath.Join(apiDir, "buf.yaml"))
	if string(content) != customContent {
		t.Error("buf.yaml 被覆盖，期望保留自定义内容")
	}
}

// TestFindBufGenYaml 测试 buf.gen.yaml 查找逻辑.
func TestFindBufGenYaml(t *testing.T) {
	dir := t.TempDir()

	// 创建目录结构: dir/api/order/v1/
	apiDir := filepath.Join(dir, "api")
	protoDir := filepath.Join(apiDir, "order", "v1")
	os.MkdirAll(protoDir, 0o755)

	protoFile := filepath.Join(protoDir, "order.proto")
	os.WriteFile(protoFile, []byte(`syntax = "proto3";`), 0o644)

	// 无 buf.gen.yaml 时应返回空
	if got := findBufGenYaml(protoFile); got != "" {
		t.Errorf("findBufGenYaml = %q, 期望返回空", got)
	}

	// 在 api/ 下创建 buf.gen.yaml
	genYaml := filepath.Join(apiDir, "buf.gen.yaml")
	os.WriteFile(genYaml, []byte("version: v2"), 0o644)

	got := findBufGenYaml(protoFile)
	if got == "" {
		t.Fatal("findBufGenYaml 返回空，期望找到 buf.gen.yaml")
	}
}

// TestFindAPIDir 测试 api 目录查找.
func TestFindAPIDir(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	protoDir := filepath.Join(apiDir, "order", "v1")
	os.MkdirAll(protoDir, 0o755)

	protoFile := filepath.Join(protoDir, "order.proto")
	got := findAPIDir(protoFile)
	if filepath.Base(got) != "api" {
		t.Errorf("findAPIDir = %q, 期望以 'api' 结尾", got)
	}
}

// TestGenerateBufYaml 测试 buf.yaml 生成.
func TestGenerateBufYaml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "buf.yaml")

	if err := generateBufYaml(path); err != nil {
		t.Fatalf("generateBufYaml 失败: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 buf.yaml 失败: %v", err)
	}
	text := string(content)
	if !contains(text, "version: v2") {
		t.Error("缺少 version: v2")
	}
	if !contains(text, "STANDARD") {
		t.Error("缺少 STANDARD lint 规则")
	}
}

// TestGenerateBufGenYaml 测试 buf.gen.yaml 生成.
func TestGenerateBufGenYaml(t *testing.T) {
	dir := t.TempDir()

	// 不含 HTTP
	path := filepath.Join(dir, "basic.yaml")
	if err := generateBufGenYaml(path, false); err != nil {
		t.Fatalf("generateBufGenYaml(false) 失败: %v", err)
	}
	content, _ := os.ReadFile(path)
	text := string(content)
	if !contains(text, "buf.build/protocolbuffers/go") {
		t.Error("缺少 protoc-gen-go 插件")
	}
	if contains(text, "gateway") {
		t.Error("基础模板不应包含 gateway 插件")
	}

	// 含 HTTP
	pathHTTP := filepath.Join(dir, "http.yaml")
	if err := generateBufGenYaml(pathHTTP, true); err != nil {
		t.Fatalf("generateBufGenYaml(true) 失败: %v", err)
	}
	content, _ = os.ReadFile(pathHTTP)
	text = string(content)
	if !contains(text, "gateway") {
		t.Error("HTTP 模板应包含 gateway 插件")
	}
}

// TestRunProtoLintNoArgs 测试 proto lint 默认参数.
func TestRunProtoLintNoArgs(t *testing.T) {
	// 不存在 api/ 目录时应报错
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(t.TempDir())

	err := runProtoLint(nil)
	if err == nil {
		t.Error("api/ 目录不存在时应返回错误")
	}
}

// TestRunProtoBreakingNoArgs 测试 proto breaking 默认参数.
func TestRunProtoBreakingNoArgs(t *testing.T) {
	// 不存在 api/ 目录时应报错
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(t.TempDir())

	err := runProtoBreaking(nil)
	if err == nil {
		t.Error("api/ 目录不存在时应返回错误")
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
		t.Fatalf("写入 proto 文件失败: %v", err)
	}

	serviceName, rpcs, err := parseProtoFile(protoPath)
	if err != nil {
		t.Fatalf("parseProtoFile 失败: %v", err)
	}

	if serviceName != "Order" {
		t.Errorf("serviceName = %q, 期望 %q", serviceName, "Order")
	}
	if len(rpcs) != 3 {
		t.Fatalf("rpcs 数量 = %d, 期望 3", len(rpcs))
	}

	if rpcs[0].MethodName != "CreateOrder" {
		t.Errorf("rpcs[0].MethodName = %q, 期望 %q", rpcs[0].MethodName, "CreateOrder")
	}
	if rpcs[0].RequestType != "CreateOrderRequest" {
		t.Errorf("rpcs[0].RequestType = %q, 期望 %q", rpcs[0].RequestType, "CreateOrderRequest")
	}
	if rpcs[0].ReplyType != "CreateOrderReply" {
		t.Errorf("rpcs[0].ReplyType = %q, 期望 %q", rpcs[0].ReplyType, "CreateOrderReply")
	}
}

// TestRunProtoServer 测试 proto server 生成 Go 服务桩代码.
func TestRunProtoServer(t *testing.T) {
	dir := t.TempDir()

	// 先生成 proto 文件
	err := runProtoAdd([]string{"order", "--module", "github.com/example/myapp", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd 失败: %v", err)
	}

	protoPath := filepath.Join(dir, "api/order/v1/order.proto")
	targetDir := filepath.Join(dir, "internal/service")

	err = runProtoServer([]string{protoPath, "--target", targetDir})
	if err != nil {
		t.Fatalf("runProtoServer 失败: %v", err)
	}

	outPath := filepath.Join(targetDir, "order.go")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读取服务端桩代码失败: %v", err)
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
			t.Errorf("服务端桩代码缺少 %s: 期望包含 %q", c.desc, c.substr)
		}
	}
}

// TestRunProtoServerNoArgs 测试无参数调用 proto server.
func TestRunProtoServerNoArgs(t *testing.T) {
	err := runProtoServer(nil)
	if err == nil {
		t.Error("缺少 proto 文件参数时应返回错误")
	}
}

// TestHasHTTPAnnotations 测试 HTTP 注解检测.
func TestHasHTTPAnnotations(t *testing.T) {
	dir := t.TempDir()

	// 包含注解
	withAnnotations := filepath.Join(dir, "with.proto")
	if err := os.WriteFile(withAnnotations, []byte(`import "google/api/annotations.proto";`), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
	if !hasHTTPAnnotations(withAnnotations) {
		t.Error("包含注解的 proto 文件应返回 true")
	}

	// 不包含注解
	withoutAnnotations := filepath.Join(dir, "without.proto")
	if err := os.WriteFile(withoutAnnotations, []byte(`syntax = "proto3";`), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
	if hasHTTPAnnotations(withoutAnnotations) {
		t.Error("不含注解的 proto 文件应返回 false")
	}

	// 文件不存在
	if hasHTTPAnnotations(filepath.Join(dir, "nonexistent.proto")) {
		t.Error("不存在的文件应返回 false")
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
		t.Fatalf("写入文件失败: %v", err)
	}

	mod, err := detectModuleFromProto(protoPath)
	if err != nil {
		t.Fatalf("detectModuleFromProto 失败: %v", err)
	}
	if mod != "github.com/example/myapp" {
		t.Errorf("module = %q, 期望 %q", mod, "github.com/example/myapp")
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
		t.Fatalf("创建 services 目录失败: %v", err)
	}

	// 先生成 proto 文件
	err := runProtoAdd([]string{"order", "--module", "github.com/example/monorepo", "--output", dir})
	if err != nil {
		t.Fatalf("runProtoAdd 失败: %v", err)
	}

	protoPath := filepath.Join(dir, "api/order/v1/order.proto")

	// 使用 --service 标志触发 monorepo 模式
	err = runProtoServer([]string{protoPath, "--service", "order"})
	if err != nil {
		t.Fatalf("runProtoServer --service 失败: %v", err)
	}

	// 验证输出到 services/order-service/internal/service/order.go
	outPath := filepath.Join("services", "order-service", "internal", "service", "order.go")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("monorepo 模式下期望桩代码在 %q", outPath)
	}
}

// TestBuildProtoServerArgs 测试 proto server 参数构建.
func TestBuildProtoServerArgs(t *testing.T) {
	// 默认参数: 仅 proto 文件
	args := buildProtoServerArgs("test.proto", "internal/service", "")
	if len(args) != 1 || args[0] != "test.proto" {
		t.Errorf("默认参数 = %v, 期望 [test.proto]", args)
	}

	// 自定义 target
	args = buildProtoServerArgs("test.proto", "custom/path", "")
	if len(args) != 3 {
		t.Fatalf("含 target 参数: 长度 = %d, 期望 3", len(args))
	}
	if args[1] != "-target" || args[2] != "custom/path" {
		t.Errorf("含 target 参数 = %v, 期望 [test.proto -target custom/path]", args)
	}

	// 带 service 标志
	args = buildProtoServerArgs("test.proto", "internal/service", "order")
	if len(args) != 3 {
		t.Fatalf("含 service 参数: 长度 = %d, 期望 3", len(args))
	}
	if args[1] != "-service" || args[2] != "order" {
		t.Errorf("含 service 参数 = %v, 期望 [test.proto -service order]", args)
	}
}
