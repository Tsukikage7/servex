package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/project/*
var projectTemplates embed.FS

//go:embed templates/monorepo/*
var monorepoTemplates embed.FS

// ProjectData 项目模板数据.
type ProjectData struct {
	Name     string
	Module   string
	WithGRPC bool
	WithWire bool
	Infra    []InfraDef // 基础设施组件列表
}

// runNew 执行 servex new 命令.
func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	module := fs.String("module", "", "Go module 路径 (默认: github.com/example/<project>)")
	standalone := fs.Bool("standalone", false, "创建独立单服务项目[默认: monorepo 模式]")
	withGRPC := fs.Bool("with-grpc", false, "包含 gRPC 服务端")
	infra := fs.String("infra", "", "基础设施组件，逗号分隔 (如 mysql,redis,kafka)")
	withWire := fs.Bool("with-wire", false, "包含 Wire 依赖注入")
	fs.Usage = func() {
		fmt.Println("用法: servex new <project-name> [options]")
		fmt.Println()
		fmt.Println("选项:")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("项目名称必填")
	}

	projectName := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *module == "" {
		*module = "github.com/example/" + projectName
	}

	data := ProjectData{
		Name:     projectName,
		Module:   *module,
		WithGRPC: *withGRPC,
		WithWire: *withWire,
		Infra:    parseInfra(*infra),
	}

	if *standalone {
		return generateProject(data)
	}
	return generateMonorepo(data)
}

// templateFiles 定义项目模板文件与输出路径的映射.
var templateFiles = []struct {
	tmpl string // 模板路径[embed.FS 内]
	out  string // 输出路径[相对于项目根目录]，支持模板变量
}{
	{"templates/project/go.mod.tmpl", "go.mod"},
	{"templates/project/main.go.tmpl", "README.md"},
	{"templates/project/config.yaml.tmpl", "config.yaml"},
	{"templates/project/justfile.tmpl", "justfile"},
	{"templates/project/Dockerfile.tmpl", "Dockerfile"},
	{"templates/project/.gitignore.tmpl", ".gitignore"},
	{"templates/project/cmd/server/main.go.tmpl", "cmd/server/main.go"},
	{"templates/project/internal/server/http.go.tmpl", "internal/server/http.go"},
	{"templates/project/internal/service/service.go.tmpl", "internal/service/service.go"},
}

// grpcTemplateFile gRPC 可选模板.
var grpcTemplateFile = struct {
	tmpl string
	out  string
}{
	"templates/project/internal/server/grpc.go.tmpl", "internal/server/grpc.go",
}

// wireTemplateFile Wire DI 可选模板.
var wireTemplateFile = struct {
	tmpl string
	out  string
}{
	"templates/project/cmd/server/wire.go.tmpl", "cmd/server/wire.go",
}

// generateProject 根据模板数据生成独立项目文件.
func generateProject(data ProjectData) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
		"extraImports": extraImports,
	}

	// 生成标准模板文件
	for _, tf := range templateFiles {
		if err := renderTemplate(projectTemplates, tf.tmpl, filepath.Join(data.Name, tf.out), data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	// 生成 gRPC 模板[可选]
	if data.WithGRPC {
		tf := grpcTemplateFile
		if err := renderTemplate(projectTemplates, tf.tmpl, filepath.Join(data.Name, tf.out), data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	// 生成 Wire 模板[可选]
	if data.WithWire {
		tf := wireTemplateFile
		if err := renderTemplate(projectTemplates, tf.tmpl, filepath.Join(data.Name, tf.out), data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	fmt.Printf("项目 %q 创建成功! (standalone)\n", data.Name)
	fmt.Printf("  cd %s && go mod tidy\n", data.Name)
	return nil
}

// monorepoTemplateFiles 定义 monorepo 模板文件与输出路径的映射.
var monorepoTemplateFiles = []struct {
	tmpl string // 模板路径[embed.FS 内]
	out  string // 输出路径[相对于项目根目录]
}{
	{"templates/monorepo/go.mod.tmpl", "go.mod"},
	{"templates/monorepo/justfile.tmpl", "justfile"},
	{"templates/monorepo/.gitignore.tmpl", ".gitignore"},
	{"templates/monorepo/README.md.tmpl", "README.md"},
}

// monorepoGitkeepDirs monorepo 需要创建的 .gitkeep 目录列表.
var monorepoGitkeepDirs = []string{
	"domain",
	"application",
	"services",
	"api",
	"infrastructure",
	"deploy/docker",
	"deploy/k8s",
}

// generateMonorepo 根据模板数据生成 monorepo 项目结构.
func generateMonorepo(data ProjectData) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
		"extraImports": extraImports,
	}

	// 生成模板文件
	for _, tf := range monorepoTemplateFiles {
		if err := renderTemplate(monorepoTemplates, tf.tmpl, filepath.Join(data.Name, tf.out), data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	// 创建 .gitkeep 目录
	for _, dir := range monorepoGitkeepDirs {
		gitkeepPath := filepath.Join(data.Name, dir, ".gitkeep")
		if err := os.MkdirAll(filepath.Dir(gitkeepPath), 0o755); err != nil {
			return fmt.Errorf("创建目录 %s: %w", dir, err)
		}
		if err := os.WriteFile(gitkeepPath, nil, 0o644); err != nil {
			return fmt.Errorf("创建 .gitkeep: %s: %w", dir, err)
		}
	}

	fmt.Printf("项目 %q 创建成功! (monorepo)\n", data.Name)
	fmt.Printf("  cd %s && go mod tidy\n", data.Name)
	fmt.Println("  servex add service <name> --with-grpc --infra mysql,redis")
	return nil
}

// renderTemplate 渲染单个模板文件到指定路径.
func renderTemplate(fsys embed.FS, tmplPath, outPath string, data any, funcMap template.FuncMap) error {
	content, err := fsys.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("读取模板 %s: %w", tmplPath, err)
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Funcs(funcMap).Parse(string(content))
	if err != nil {
		return fmt.Errorf("解析模板 %s: %w", tmplPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("创建目录 %s: %w", filepath.Dir(outPath), err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("创建文件 %s: %w", outPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("执行模板 %s: %w", tmplPath, err)
	}

	return nil
}
