package main

import (
	"embed"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/gen/*
var genTemplates embed.FS

//go:embed templates/aggregate/*
var aggregateTemplates embed.FS

// Field 聚合字段定义.
type Field struct {
	Name      string // PascalCase 字段名
	Type      string // Go 类型
	JSONTag   string // JSON tag (snake_case)
	CamelName string // camelCase 字段名
}

// DockerData Dockerfile 模板数据.
type DockerData struct {
	Name string // 服务名称
	Port string // 暴露端口
}

// JustfileData justfile 模板数据.
type JustfileData struct {
	Name   string // 服务名称
	Module string // Go module 路径
}

// AggregateData 聚合模板数据.
type AggregateData struct {
	Name        string  // PascalCase 名称
	NameLower   string  // camelCase 名称 (包名)
	NameSnake   string  // snake_case 名称
	Module      string  // Go module 路径
	Fields      []Field // 所有字段（含 ID）
	NonIDFields []Field // 非 ID 字段
	IDType      string  // ID 字段类型
	NeedsTime   bool
}

// runGen 执行 servex gen 命令.
func runGen(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: servex gen <type> [arguments]")
		fmt.Println()
		fmt.Println("Types:")
		fmt.Println("  aggregate    Generate DDD aggregate boilerplate")
		fmt.Println("  dockerfile   Generate a Dockerfile")
		fmt.Println("  justfile     Generate a justfile")
		return fmt.Errorf("gen type is required")
	}

	switch args[0] {
	case "aggregate":
		return runGenAggregate(args[1:])
	case "dockerfile":
		return runGenDockerfile(args[1:])
	case "justfile":
		return runGenJustfile(args[1:])
	default:
		return fmt.Errorf("unknown gen type: %s", args[0])
	}
}

// runGenAggregate 执行 servex gen aggregate 命令.
func runGenAggregate(args []string) error {
	fs := flag.NewFlagSet("gen aggregate", flag.ExitOnError)
	fieldsStr := fs.String("fields", "", `Field definitions (e.g. "id:uint64,name:string,email:string")`)
	output := fs.String("output", ".", "Output base directory")
	fs.Usage = func() {
		fmt.Println("Usage: servex gen aggregate <name> [options]")
		fmt.Println()
		fmt.Println("Generates DDD aggregate files:")
		fmt.Println("  domain/<name>/aggregate.go    Aggregate root")
		fmt.Println("  domain/<name>/event.go        Domain events")
		fmt.Println("  domain/<name>/repository.go   Repository interface (port)")
		fmt.Println("  domain/<name>/command.go       CQRS commands")
		fmt.Println("  domain/<name>/query.go         CQRS queries & view")
		fmt.Println("  application/<name>/service.go  Application service")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("aggregate name is required")
	}

	name := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	fields := parseFields(*fieldsStr)
	module := ""
	if m, err := detectModule(); err == nil {
		module = m
	}
	data := buildAggregateData(name, module, fields)

	return generateAggregate(data, *output)
}

// parseFields 解析字段定义字符串.
func parseFields(s string) []Field {
	if s == "" {
		return nil
	}

	var fields []Field
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		name := strings.TrimSpace(kv[0])
		typ := "string"
		if len(kv) == 2 {
			typ = strings.TrimSpace(kv[1])
		}
		fields = append(fields, Field{
			Name:      toPascalCase(name),
			Type:      goType(typ),
			JSONTag:   toSnakeCase(name),
			CamelName: toCamelCase(name),
		})
	}
	return fields
}

// buildAggregateData 从字段列表构建聚合模板数据.
func buildAggregateData(name, module string, fields []Field) AggregateData {
	idType := "uint64" // 默认 ID 类型
	var nonIDFields []Field

	for _, f := range fields {
		if strings.EqualFold(f.Name, "Id") || strings.EqualFold(f.Name, "ID") {
			idType = f.Type
		} else {
			nonIDFields = append(nonIDFields, f)
		}
	}

	return AggregateData{
		Name:        toPascalCase(name),
		NameLower:   strings.ToLower(toPascalCase(name)),
		NameSnake:   toSnakeCase(name),
		Module:      module,
		Fields:      fields,
		NonIDFields: nonIDFields,
		IDType:      idType,
		NeedsTime:   needsTimeImport(fields),
	}
}

// aggregateFiles 聚合模板文件映射.
// dir: "domain" 或 "application", tmpl: 模板路径, out: 输出文件名.
var aggregateFiles = []struct {
	dir  string // 输出子目录前缀
	tmpl string // 模板路径
	out  string // 输出文件名
}{
	{"domain", "templates/aggregate/aggregate.go.tmpl", "aggregate.go"},
	{"domain", "templates/aggregate/event.go.tmpl", "event.go"},
	{"domain", "templates/aggregate/repository.go.tmpl", "repository.go"},
	{"domain", "templates/aggregate/command.go.tmpl", "command.go"},
	{"domain", "templates/aggregate/query.go.tmpl", "query.go"},
	{"application", "templates/aggregate/service.go.tmpl", "service.go"},
}

// generateAggregate 根据模板数据生成 DDD 聚合文件.
func generateAggregate(data AggregateData, outputDir string) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
	}

	for _, af := range aggregateFiles {
		outPath := filepath.Join(outputDir, af.dir, data.NameLower, af.out)
		if err := renderTemplate(aggregateTemplates, af.tmpl, outPath, data, funcMap); err != nil {
			return fmt.Errorf("render %s: %w", af.tmpl, err)
		}
	}

	fmt.Printf("DDD aggregate %q generated", data.Name)
	if isMonorepo() {
		fmt.Println(" (monorepo detected):")
	} else {
		fmt.Println(":")
	}
	fmt.Printf("  domain/%s/      (aggregate, events, repository, commands, queries)\n", data.NameLower)
	fmt.Printf("  application/%s/ (service)\n", data.NameLower)
	return nil
}

// runGenDockerfile 执行 servex gen dockerfile 命令.
func runGenDockerfile(args []string) error {
	fs := flag.NewFlagSet("gen dockerfile", flag.ExitOnError)
	name := fs.String("name", "server", "Service name")
	port := fs.String("port", "8080", "Exposed port")
	output := fs.String("output", ".", "Output directory")
	fs.Usage = func() {
		fmt.Println("Usage: servex gen dockerfile [options]")
		fmt.Println()
		fmt.Println("Generates a multi-stage Dockerfile for Go services.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	data := DockerData{
		Name: *name,
		Port: *port,
	}

	return generateDockerfile(data, *output)
}

// generateDockerfile 根据模板数据生成 Dockerfile.
func generateDockerfile(data DockerData, outputDir string) error {
	outPath := filepath.Join(outputDir, "Dockerfile")
	if err := renderTemplate(genTemplates, "templates/gen/Dockerfile.tmpl", outPath, data, nil); err != nil {
		return fmt.Errorf("render Dockerfile: %w", err)
	}

	fmt.Printf("Dockerfile generated: %s\n", outPath)
	return nil
}

// runGenJustfile 执行 servex gen justfile 命令.
func runGenJustfile(args []string) error {
	fs := flag.NewFlagSet("gen justfile", flag.ExitOnError)
	name := fs.String("name", "server", "Service name")
	module := fs.String("module", "", "Go module path")
	output := fs.String("output", ".", "Output directory")
	fs.Usage = func() {
		fmt.Println("Usage: servex gen justfile [options]")
		fmt.Println()
		fmt.Println("Generates a justfile with build, test, lint, proto, wire, and docker recipes.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *module == "" {
		*module = "github.com/example/" + *name
	}

	data := JustfileData{
		Name:   *name,
		Module: *module,
	}

	return generateJustfile(data, *output)
}

// generateJustfile 根据模板数据生成 justfile.
func generateJustfile(data JustfileData, outputDir string) error {
	outPath := filepath.Join(outputDir, "justfile")
	if err := renderTemplate(genTemplates, "templates/gen/justfile.tmpl", outPath, data, nil); err != nil {
		return fmt.Errorf("render justfile: %w", err)
	}

	fmt.Printf("justfile generated: %s\n", outPath)
	return nil
}
