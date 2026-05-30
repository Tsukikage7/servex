package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed all:templates/project
var projectTemplates embed.FS

//go:embed all:templates/monorepo
var monorepoTemplates embed.FS

//go:embed all:templates/ai-service
var aiServiceTemplates embed.FS

// ProjectData 项目模板数据.
type ProjectData struct {
	Name          string
	Module        string
	WithGRPC      bool
	Infra         []ComponentDef // 基础设施组件列表
	AllComponents []ComponentDef // 合并后的全部组件[模板用]
}

// AIServiceData AI service 模板数据.
type AIServiceData struct {
	Name      string
	Module    string
	Provider  string
	Framework string
}

// templateFiles 定义项目模板文件与输出路径的映射.
var templateFiles = []struct {
	tmpl string // 模板路径[embed.FS 内]
	out  string // 输出路径[相对于项目根目录]，支持模板变量
}{
	{"templates/project/go.mod.tmpl", "go.mod"},
	{"templates/project/readme.md.tmpl", "README.md"},
	{"templates/project/config.yaml.tmpl", "config.yaml"},
	{"templates/project/justfile.tmpl", "justfile"},
	{"templates/project/Dockerfile.tmpl", "Dockerfile"},
	{"templates/project/.gitignore.tmpl", ".gitignore"},
	{"templates/project/cmd/server/main.go.tmpl", "cmd/server/main.go"},
	{"templates/project/cmd/server/wire.go.tmpl", "cmd/server/wire.go"},
	{"templates/project/cmd/server/provider.go.tmpl", "cmd/server/provider.go"},
	{"templates/project/cmd/server/config.go.tmpl", "cmd/server/config.go"},
	{"templates/project/internal/server/http.go.tmpl", "internal/server/http.go"},
	{"templates/project/internal/service/service.go.tmpl", "internal/service/service.go"},
	{"templates/project/deploy/docker/infra/docker-compose.yaml.tmpl", "deploy/docker/infra/docker-compose.yaml"},
	{"templates/project/deploy/docker/app/docker-compose.yaml.tmpl", "deploy/docker/app/docker-compose.yaml"},
}

// grpcTemplateFile gRPC 可选模板.
var grpcTemplateFile = struct {
	tmpl string
	out  string
}{
	"templates/project/internal/server/grpc.go.tmpl", "internal/server/grpc.go",
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

	// 复制 api/third_partyvendor proto 文件，不需要模板渲染
	if err := copyEmbedDir(projectTemplates, "templates/project/api/third_party", filepath.Join(data.Name, "api/third_party")); err != nil {
		return fmt.Errorf("复制 third_party: %w", err)
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
	{"templates/monorepo/api/buf.yaml.tmpl", "api/buf.yaml"},
	{"templates/monorepo/api/buf.gen.yaml.tmpl", "api/buf.gen.yaml"},
	{"templates/monorepo/docs/README.md.tmpl", "docs/README.md"},
	{"templates/monorepo/deploy/docker/infra/docker-compose.yaml.tmpl", "deploy/docker/infra/docker-compose.yaml"},
	{"templates/monorepo/deploy/docker/app/docker-compose.yaml.tmpl", "deploy/docker/app/docker-compose.yaml"},
}

// monorepoGitkeepDirs monorepo 需要创建的 .gitkeep 目录列表.
var monorepoGitkeepDirs = []string{
	"domain",
	"application",
	"services",
	"api",
	"infrastructure",
	"deploy/docker/base",
	"deploy/docker/app",
	"deploy/docker/infra",
	"deploy/k8s",
	"docs/development",
	"docs/operations",
	"docs/product",
	"scripts/build",
	"scripts/dev",
	"scripts/deploy",
	"scripts/quality",
	"scripts/ops",
	"bin",
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

	// 复制 api/third_partyvendor proto 文件，不需要模板渲染
	if err := copyEmbedDir(monorepoTemplates, "templates/monorepo/api/third_party", filepath.Join(data.Name, "api/third_party")); err != nil {
		return fmt.Errorf("复制 third_party: %w", err)
	}

	fmt.Printf("项目 %q 创建成功! (monorepo)\n", data.Name)
	fmt.Printf("  cd %s && go mod tidy\n", data.Name)
	fmt.Println("  servex add service <name> --with-grpc --infra mysql,redis")
	return nil
}

var aiServiceTemplateFiles = []struct {
	tmpl string
	out  string
}{
	{"templates/ai-service/go.mod.tmpl", "go.mod"},
	{"templates/ai-service/README.md.tmpl", "README.md"},
	{"templates/ai-service/justfile.tmpl", "justfile"},
	{"templates/ai-service/configs/config.yaml.tmpl", "configs/config.yaml"},
	{"templates/ai-service/cmd/server/main.go.tmpl", "cmd/server/main.go"},
	{"templates/ai-service/internal/agent/agent.go.tmpl", "internal/agent/agent.go"},
	{"templates/ai-service/internal/http/chat.go.tmpl", "internal/http/chat.go"},
	{"templates/ai-service/internal/llm/model.go.tmpl", "internal/llm/model.go"},
	{"templates/ai-service/internal/tools/tools.go.tmpl", "internal/tools/tools.go"},
}

// generateAIService 根据模板数据生成 AI service 项目结构.
func generateAIService(data AIServiceData) error {
	if data.Provider == "" {
		data.Provider = "openai"
	}
	if data.Framework == "" {
		data.Framework = "none"
	}

	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
	}

	for _, tf := range aiServiceTemplateFiles {
		if err := renderTemplate(aiServiceTemplates, tf.tmpl, filepath.Join(data.Name, tf.out), data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", tf.tmpl, err)
		}
	}

	fmt.Printf("AI service %q 创建成功!\n", data.Name)
	fmt.Printf("  cd %s && go mod tidy\n", data.Name)
	fmt.Println("  OPENAI_API_KEY=sk-... just dev")
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

// copyEmbedDir 将 embed.FS 中的目录递归复制到目标路径原样复制，不做模板渲染.
func copyEmbedDir(fsys embed.FS, srcDir, dstDir string) error {
	entries, err := fsys.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取目录 %s: %w", srcDir, err)
	}

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("创建目录 %s: %w", dstDir, err)
	}

	for _, entry := range entries {
		srcPath := srcDir + "/" + entry.Name()
		dstPath := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			if err := copyEmbedDir(fsys, srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		data, err := fsys.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("读取文件 %s: %w", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return fmt.Errorf("写入文件 %s: %w", dstPath, err)
		}
	}
	return nil
}
