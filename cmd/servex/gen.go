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

// CommandDef 业务命令定义.
type CommandDef struct {
	Name      string // PascalCase: "Place", "Cancel", "Ship"
	EventName string // "Placed", "Cancelled", "Shipped" (past tense)
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
	Name         string       // PascalCase 名称
	NameLower    string       // camelCase 名称 (包名)
	NameSnake    string       // snake_case 名称
	Module       string       // Go module 路径
	Service      string       // 目标服务名[monorepo 模式下生成 adapter 层]
	Fields       []Field      // 所有字段[含 ID]
	NonIDFields  []Field      // 非 ID 字段
	IDType       string       // ID 字段类型
	NeedsTime    bool
	Commands     []CommandDef // 业务命令列表
	UniqueFields []Field      // 唯一字段[生成 FindByXxx]
	HasCommands  bool         // 是否指定了 --commands
}

// runGen 执行 servex gen 命令.
func runGen(args []string) error {
	if len(args) == 0 {
		fmt.Println("用法: servex gen <type> [arguments]")
		fmt.Println()
		fmt.Println("类型:")
		fmt.Println("  aggregate    生成 DDD 聚合代码")
		fmt.Println("  dockerfile   生成 Dockerfile")
		fmt.Println("  justfile     生成 justfile")
		fmt.Println("  k8s          生成 K8s 清单[Deployment/Service/HPA]")
		return fmt.Errorf("必须指定生成类型")
	}

	switch args[0] {
	case "aggregate":
		return runGenAggregate(args[1:])
	case "dockerfile":
		return runGenDockerfile(args[1:])
	case "justfile":
		return runGenJustfile(args[1:])
	case "k8s":
		return runGenK8sLegacy(args[1:])
	default:
		return fmt.Errorf("未知生成类型: %s", args[0])
	}
}

// runGenAggregate 执行 servex gen aggregate 命令.
func runGenAggregate(args []string) error {
	fs := flag.NewFlagSet("gen aggregate", flag.ExitOnError)
	fieldsStr := fs.String("fields", "", `字段定义 (如 "id:uint64,name:string,email:string")`)
	commandsStr := fs.String("commands", "", `业务命令 (如 "Place,Cancel,Ship")`)
	uniqueStr := fs.String("unique", "", `唯一字段，生成 FindByXxx (如 "email,username")`)
	output := fs.String("output", ".", "输出目录")
	fs.Usage = func() {
		fmt.Println("用法: servex gen aggregate <name> [options]")
		fmt.Println()
		fmt.Println("生成 DDD 聚合文件:")
		fmt.Println("  domain/<name>/aggregate.go         聚合根")
		fmt.Println("  domain/<name>/event.go             领域事件")
		fmt.Println("  domain/<name>/repository.go        仓储接口 (端口)")
		fmt.Println("  domain/<name>/command.go            CQRS 命令")
		fmt.Println("  domain/<name>/query.go              CQRS 查询 & 视图")
		fmt.Println("  domain/<name>/aggregate_test.go    聚合单元测试")
		fmt.Println("  domain/<name>/repository_mock.go   仓储 Mock")
		fmt.Println("  application/<name>/service.go      应用服务")
		fmt.Println("  application/<name>/service_test.go 服务单元测试")
		fmt.Println()
		fmt.Println("选项:")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("必须指定聚合名称")
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
	data := buildAggregateData(name, module, fields, *commandsStr, *uniqueStr)

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

// parseCommands 解析业务命令定义字符串.
func parseCommands(s string) []CommandDef {
	if s == "" {
		return nil
	}

	var cmds []CommandDef
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := toPascalCase(part)
		cmds = append(cmds, CommandDef{
			Name:      name,
			EventName: toPastTense(name),
		})
	}
	return cmds
}

// toPastTense 将动词转为过去式.
// Place→Placed, Cancel→Cancelled, Ship→Shipped.
func toPastTense(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "e") {
		return s + "d"
	}
	// 双写末尾辅音 + ed 的常见模式：以单元音+单辅音结尾的短音节
	n := len(lower)
	if n >= 2 && isConsonant(lower[n-1]) && isVowel(lower[n-2]) {
		return s + string(s[len(s)-1]) + "ed"
	}
	return s + "ed"
}

func isVowel(c byte) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}

func isConsonant(c byte) bool {
	return c >= 'a' && c <= 'z' && !isVowel(c)
}

// parseUniqueFields 从字段名匹配已有字段列表.
func parseUniqueFields(s string, allFields []Field) []Field {
	if s == "" {
		return nil
	}

	lookup := make(map[string]Field)
	for _, f := range allFields {
		lookup[strings.ToLower(f.Name)] = f
		lookup[strings.ToLower(f.CamelName)] = f
		lookup[strings.ToLower(f.JSONTag)] = f
	}

	var result []Field
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if f, ok := lookup[strings.ToLower(part)]; ok {
			result = append(result, f)
		}
	}
	return result
}

// buildAggregateData 从字段列表构建聚合模板数据.
func buildAggregateData(name, module string, fields []Field, commandsStr, uniqueStr string) AggregateData {
	idType := "uint64" // 默认 ID 类型
	var nonIDFields []Field

	for _, f := range fields {
		if strings.EqualFold(f.Name, "Id") || strings.EqualFold(f.Name, "ID") {
			idType = f.Type
		} else {
			nonIDFields = append(nonIDFields, f)
		}
	}

	commands := parseCommands(commandsStr)
	uniqueFields := parseUniqueFields(uniqueStr, nonIDFields)

	return AggregateData{
		Name:         toPascalCase(name),
		NameLower:    strings.ToLower(toPascalCase(name)),
		NameSnake:    toSnakeCase(name),
		Module:       module,
		Fields:       fields,
		NonIDFields:  nonIDFields,
		IDType:       idType,
		NeedsTime:    needsTimeImport(fields),
		Commands:     commands,
		UniqueFields: uniqueFields,
		HasCommands:  len(commands) > 0,
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
	{"domain", "templates/aggregate/aggregate_test.go.tmpl", "aggregate_test.go"},
	{"domain", "templates/aggregate/repository_mock.go.tmpl", "repository_mock.go"},
	{"application", "templates/aggregate/service.go.tmpl", "service.go"},
	{"application", "templates/aggregate/service_test.go.tmpl", "service_test.go"},
}

// adapterFiles adapter 层模板文件映射.
var adapterFiles = []struct {
	tmpl string // 模板路径
	out  string // 输出文件名
}{
	{"templates/aggregate/adapter_repo.go.tmpl", "_repo.go"},
	{"templates/aggregate/adapter_model.go.tmpl", "_model.go"},
	{"templates/aggregate/adapter_repo_test.go.tmpl", "_repo_test.go"},
}

// generateAggregate 根据模板数据生成 DDD 聚合文件.
func generateAggregate(data AggregateData, outputDir string) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
		"zeroValue":    zeroValue,
	}

	for _, af := range aggregateFiles {
		outPath := filepath.Join(outputDir, af.dir, data.NameLower, af.out)
		if err := renderTemplate(aggregateTemplates, af.tmpl, outPath, data, funcMap); err != nil {
			return fmt.Errorf("渲染 %s: %w", af.tmpl, err)
		}
	}

	fmt.Printf("DDD 聚合 %q 已生成", data.Name)
	if isMonorepo() {
		fmt.Println(" (检测到 monorepo):")
	} else {
		fmt.Println(":")
	}
	fmt.Printf("  domain/%s/      (聚合, 事件, 仓储, 命令, 查询, 测试, mock)\n", data.NameLower)
	fmt.Printf("  application/%s/ (服务, 测试)\n", data.NameLower)

	// 如果指定了 --service，生成 adapter/persistence 层
	if data.Service != "" {
		adapterBase := filepath.Join(outputDir, "services", data.Service+"-service", "internal", "adapter", "persistence")
		for _, af := range adapterFiles {
			outPath := filepath.Join(adapterBase, data.NameSnake+af.out)
			if err := renderTemplate(aggregateTemplates, af.tmpl, outPath, data, funcMap); err != nil {
				return fmt.Errorf("渲染 %s: %w", af.tmpl, err)
			}
		}
		fmt.Printf("  services/%s-service/internal/adapter/persistence/ (仓储, 模型, 测试)\n", data.Service)
	}

	return nil
}

// runGenDockerfile 执行 servex gen dockerfile 命令.
func runGenDockerfile(args []string) error {
	fs := flag.NewFlagSet("gen dockerfile", flag.ExitOnError)
	name := fs.String("name", "server", "服务名称")
	port := fs.String("port", "8080", "暴露端口")
	output := fs.String("output", ".", "输出目录")
	fs.Usage = func() {
		fmt.Println("用法: servex gen dockerfile [options]")
		fmt.Println()
		fmt.Println("生成 Go 服务多阶段 Dockerfile.")
		fmt.Println()
		fmt.Println("选项:")
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
		return fmt.Errorf("渲染 Dockerfile: %w", err)
	}

	fmt.Printf("Dockerfile 已生成: %s\n", outPath)
	return nil
}

// runGenJustfile 执行 servex gen justfile 命令.
func runGenJustfile(args []string) error {
	fs := flag.NewFlagSet("gen justfile", flag.ExitOnError)
	name := fs.String("name", "server", "服务名称")
	module := fs.String("module", "", "Go module 路径")
	output := fs.String("output", ".", "输出目录")
	fs.Usage = func() {
		fmt.Println("用法: servex gen justfile [options]")
		fmt.Println()
		fmt.Println("生成 justfile，包含 build/test/lint/proto/wire/docker 等常用任务.")
		fmt.Println()
		fmt.Println("选项:")
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
		return fmt.Errorf("渲染 justfile: %w", err)
	}

	fmt.Printf("justfile 已生成: %s\n", outPath)
	return nil
}
