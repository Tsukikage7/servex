package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/service/*
var serviceTemplates embed.FS

// ServiceData 服务模板数据.
type ServiceData struct {
	Name          string
	Module        string
	WithGRPC      bool
	WithGateway   bool
	Infra         []ComponentDef // 基础设施[存储/缓存/消息]
	Observe       []ComponentDef // 可观测性
	Auth          []ComponentDef // 认证
	Discovery     []ComponentDef // 服务发现
	Other         []ComponentDef // 其他
	AllComponents []ComponentDef // 合并后的全部组件[模板用]
}

// serviceTemplateFiles 定义服务模板文件与输出路径的映射.
var serviceTemplateFiles = []struct {
	tmpl string // 模板路径[embed.FS 内]
	out  string // 输出路径[相对于服务根目录]
}{
	{"templates/service/cmd/server/main.go.tmpl", "cmd/server/main.go"},
	{"templates/service/cmd/server/wire.go.tmpl", "cmd/server/wire.go"},
	{"templates/service/cmd/server/provider.go.tmpl", "cmd/server/provider.go"},
	{"templates/service/cmd/server/config.go.tmpl", "cmd/server/config.go"},
	{"templates/service/internal/service/service.go.tmpl", "internal/service/service.go"},
	{"templates/service/internal/port/http.go.tmpl", "internal/port/http.go"},
	{"templates/service/internal/adapter/persistence/persistence.go.tmpl", "internal/adapter/persistence/persistence.go"},
	{"templates/service/configs/config.yaml.tmpl", "configs/config.dev.yaml"},
	{"templates/service/configs/config.yaml.tmpl", "configs/config.prod.yaml"},
}

// serviceGRPCTemplateFile gRPC 可选服务模板.
var serviceGRPCTemplateFile = struct {
	tmpl string
	out  string
}{
	"templates/service/internal/port/grpc.go.tmpl", "internal/port/grpc.go",
}

// serviceGatewayTemplateFile Gateway 可选服务模板.
var serviceGatewayTemplateFile = struct {
	tmpl string
	out  string
}{
	"templates/service/internal/port/gateway.go.tmpl", "internal/port/gateway.go",
}

// serviceContextDirs 是新增服务时同步创建的根级业务上下文目录。
var serviceContextDirs = []string{
	"api/{{name}}/v1",
	"domain/{{name}}",
	"application/{{name}}/command",
	"application/{{name}}/query",
}

// isMonorepo 检查当前目录是否为 monorepo 项目.
func isMonorepo() bool {
	// 检查 services/ 目录是否存在
	if info, err := os.Stat("services"); err == nil && info.IsDir() {
		return true
	}
	// 检查 go.mod 存在且无 cmd/server/[即非单体项目]
	if _, err := os.Stat("go.mod"); err == nil {
		if _, err := os.Stat("cmd/server"); os.IsNotExist(err) {
			return true
		}
	}
	return false
}

// runAddService 在 monorepo 中添加微服务.
func runAddService(args []string) error {
	name := args[0]

	if !isMonorepo() {
		return fmt.Errorf("当前目录不是 monorepo 项目[缺少 services/ 目录或 go.mod]")
	}

	// 读取 module 路径
	module, err := detectModule()
	if err != nil {
		return fmt.Errorf("读取 go.mod 失败: %w", err)
	}

	serviceDir := serviceDir(name)

	// 检查服务目录是否已存在
	if existing, ok := serviceDirExists(name); ok {
		return fmt.Errorf("服务 %q 已存在: %s", name, existing)
	}

	infra := parseInfra(addInfra)
	observe := parseObserve(addObserve)
	auth := parseAuth(addAuth)
	disc := parseDiscovery(addDiscovery)
	other := parseOther(addOther)

	var allComponents []ComponentDef
	allComponents = append(allComponents, infra...)
	allComponents = append(allComponents, observe...)
	allComponents = append(allComponents, auth...)
	allComponents = append(allComponents, disc...)
	allComponents = append(allComponents, other...)

	data := ServiceData{
		Name:          name,
		Module:        module,
		WithGRPC:      addWithGRPC,
		WithGateway:   addWithGateway,
		Infra:         infra,
		Observe:       observe,
		Auth:          auth,
		Discovery:     disc,
		Other:         other,
		AllComponents: allComponents,
	}

	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
		"extraImports": extraImports,
	}

	// 生成标准模板文件
	for _, tf := range serviceTemplateFiles {
		outPath := filepath.Join(serviceDir, tf.out)
		if err := renderTemplate(serviceTemplates, tf.tmpl, outPath, data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	if err := createServiceContextDirs(name); err != nil {
		return err
	}

	// 生成 gRPC 模板[可选]
	if data.WithGRPC {
		tf := serviceGRPCTemplateFile
		outPath := filepath.Join(serviceDir, tf.out)
		if err := renderTemplate(serviceTemplates, tf.tmpl, outPath, data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	// 生成 Gateway 模板[可选，HTTP + gRPC 双协议]
	if data.WithGateway {
		tf := serviceGatewayTemplateFile
		outPath := filepath.Join(serviceDir, tf.out)
		if err := renderTemplate(serviceTemplates, tf.tmpl, outPath, data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	fmt.Printf("服务 %q 已创建: %s\n", name, serviceDir)
	fmt.Println("下一步:")
	fmt.Printf("  servex add aggregate %s --fields \"id:uint64,name:string\" --service %s\n", name, name)
	fmt.Printf("  servex proto add %s\n", name)
	fmt.Println("  go mod tidy")
	return nil
}

func createServiceContextDirs(name string) error {
	for _, pattern := range serviceContextDirs {
		dir := filepath.Clean(replaceServiceName(pattern, name))
		gitkeepPath := filepath.Join(dir, ".gitkeep")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建目录 %s: %w", dir, err)
		}
		if _, err := os.Stat(gitkeepPath); err == nil {
			continue
		}
		if err := os.WriteFile(gitkeepPath, nil, 0o644); err != nil {
			return fmt.Errorf("创建 .gitkeep: %s: %w", dir, err)
		}
	}
	return nil
}

func replaceServiceName(pattern, name string) string {
	return filepath.FromSlash(strings.ReplaceAll(pattern, "{{name}}", name))
}
