package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/proto/*
var protoTemplates embed.FS

// ProtoData proto 模板数据.
type ProtoData struct {
	Name       string // 原始名称 (小写)
	PascalName string // PascalCase 名称
	Module     string // Go module 路径
}

// ServerData server 模板数据.
type ServerData struct {
	Name       string    // 原始名称 (小写)
	PascalName string    // PascalCase 名称
	Module     string    // Go module 路径
	RPCs       []RPCInfo // RPC 方法列表
}

// RPCInfo RPC 方法信息.
type RPCInfo struct {
	MethodName  string // 方法名
	RequestType string // 请求类型
	ReplyType   string // 响应类型
}

// runProto 执行 servex proto 命令.
func runProto(args []string) error {
	if len(args) == 0 {
		printProtoUsage()
		return fmt.Errorf("必须指定 proto 子命令")
	}

	switch args[0] {
	case "add":
		return runProtoAdd(args[1:])
	case "client":
		return runProtoClient(args[1:])
	case "server":
		return runProtoServer(args[1:])
	case "help", "-h", "--help":
		printProtoUsage()
		return nil
	default:
		printProtoUsage()
		return fmt.Errorf("未知 proto 子命令: %s", args[0])
	}
}

// printProtoUsage 输出 proto 命令帮助信息.
func printProtoUsage() {
	fmt.Println(`用法: servex proto <subcommand> [arguments]

子命令:
  add       创建 proto 服务模板
  client    从 proto 文件生成客户端代码
  server    从 proto 文件生成服务端桩代码

运行 'servex proto <subcommand> -h' 查看更多信息.`)
}

// runProtoAdd 执行 servex proto add 命令.
func runProtoAdd(args []string) error {
	fs := flag.NewFlagSet("proto add", flag.ExitOnError)
	module := fs.String("module", "", "Go module 路径 (默认: 从 go.mod 读取)")
	output := fs.String("output", ".", "输出目录")
	fs.Usage = func() {
		fmt.Println("用法: servex proto add <name> [options]")
		fmt.Println()
		fmt.Println("创建 api/<name>/v1/<name>.proto 基础服务定义.")
		fmt.Println()
		fmt.Println("选项:")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("必须指定 proto 服务名称")
	}

	name := strings.ToLower(args[0])
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *module == "" {
		mod, err := detectModule()
		if err != nil {
			return fmt.Errorf("检测 go module 失败: %w (请使用 --module 指定)", err)
		}
		*module = mod
	}

	data := ProtoData{
		Name:       name,
		PascalName: toPascalCase(name),
		Module:     *module,
	}

	outPath := filepath.Join(*output, "api", name, "v1", name+".proto")
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
	}

	if err := renderTemplate(protoTemplates, "templates/proto/service.proto.tmpl", outPath, data, funcMap); err != nil {
		return fmt.Errorf("渲染 proto 模板: %w", err)
	}

	fmt.Printf("Proto 文件已创建: %s\n", outPath)
	fmt.Printf("  服务: %sService\n", data.PascalName)
	fmt.Printf("  包名: %s.v1\n", name)
	return nil
}

// runProtoClient 执行 servex proto client 命令.
func runProtoClient(args []string) error {
	fs := flag.NewFlagSet("proto client", flag.ExitOnError)
	output := fs.String("output", "", "输出目录 (默认: proto 文件所在目录)")
	fs.Usage = func() {
		fmt.Println("用法: servex proto client <proto-file> [options]")
		fmt.Println()
		fmt.Println("从 proto 文件使用 protoc 生成 Go 客户端代码.")
		fmt.Println("生成: *.pb.go, *_grpc.pb.go, *_http.pb.go")
		fmt.Println()
		fmt.Println("选项:")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("必须指定 proto 文件路径")
	}

	protoFile := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if _, err := os.Stat(protoFile); os.IsNotExist(err) {
		return fmt.Errorf("proto 文件不存在: %s", protoFile)
	}

	outDir := *output
	if outDir == "" {
		outDir = filepath.Dir(protoFile)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录: %w", err)
	}

	protocPath, err := exec.LookPath("protoc")
	if err != nil {
		return fmt.Errorf("未找到 protoc，请先安装:\n\n安装 protoc:\n  macOS:   brew install protobuf\n  Linux:   apt install -y protobuf-compiler\n  手动:    https://github.com/protocolbuffers/protobuf/releases\n\n安装 Go 插件:\n  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest\n  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest")
	}

	protoDir := filepath.Dir(protoFile)

	// 基础 protoc 参数
	protocArgs := []string{
		"--proto_path=" + protoDir,
		"--go_out=" + outDir,
		"--go_opt=paths=source_relative",
		"--go-grpc_out=" + outDir,
		"--go-grpc_opt=paths=source_relative",
	}

	// 检测是否包含 HTTP 注解，启用 HTTP 代码生成
	if hasHTTPAnnotations(protoFile) {
		protocArgs = append(protocArgs,
			"--go-http_out="+outDir,
			"--go-http_opt=paths=source_relative",
		)
	}

	protocArgs = append(protocArgs, protoFile)

	cmd := exec.Command(protocPath, protocArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("protoc 执行失败: %w", err)
	}

	fmt.Printf("客户端代码已生成: %s\n", outDir)
	return nil
}

// hasHTTPAnnotations 检测 proto 文件是否包含 google.api.http 注解.
func hasHTTPAnnotations(protoFile string) bool {
	f, err := os.Open(protoFile)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "google/api/annotations.proto") {
			return true
		}
	}
	return false
}

// runProtoServer 执行 servex proto server 命令.
func runProtoServer(args []string) error {
	fs := flag.NewFlagSet("proto server", flag.ExitOnError)
	target := fs.String("target", "internal/service", "输出目录")
	service := fs.String("service", "", "目标服务名[monorepo 模式]")
	fs.Usage = func() {
		fmt.Println("用法: servex proto server <proto-file> [options]")
		fmt.Println()
		fmt.Println("从 proto 文件生成 Go 服务端桩代码.")
		fmt.Println()
		fmt.Println("选项:")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("必须指定 proto 文件路径")
	}

	protoFile := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if _, err := os.Stat(protoFile); os.IsNotExist(err) {
		return fmt.Errorf("proto 文件不存在: %s", protoFile)
	}

	serviceName, rpcs, err := parseProtoFile(protoFile)
	if err != nil {
		return fmt.Errorf("解析 proto 文件: %w", err)
	}

	modulePath, err := detectModuleFromProto(protoFile)
	if err != nil {
		return fmt.Errorf("检测 module: %w", err)
	}

	name := strings.ToLower(serviceName)
	data := ServerData{
		Name:       name,
		PascalName: serviceName,
		Module:     modulePath,
		RPCs:       rpcs,
	}

	// monorepo 模式: 若指定 --service 且检测到 monorepo，输出到 services/<service>-service/internal/service/
	outDir := *target
	if *service != "" && isMonorepo() {
		outDir = filepath.Join("services", *service+"-service", "internal", "service")
	}

	outPath := filepath.Join(outDir, name+".go")
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
	}

	if err := renderTemplate(protoTemplates, "templates/proto/server.go.tmpl", outPath, data, funcMap); err != nil {
		return fmt.Errorf("渲染服务端模板: %w", err)
	}

	fmt.Printf("服务端桩代码已生成: %s\n", outPath)
	fmt.Printf("  服务: %sService\n", data.PascalName)
	fmt.Printf("  方法数: %d\n", len(rpcs))
	return nil
}

// rpcPattern 匹配 proto 文件中 rpc 方法定义.
var rpcPattern = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(\w+)\s*\)`)

// servicePattern 匹配 proto 文件中 service 名称.
var servicePattern = regexp.MustCompile(`service\s+(\w+)Service\s*\{`)

// parseProtoFile 解析 proto 文件提取 service 名称和 RPC 方法.
func parseProtoFile(path string) (string, []RPCInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}

	text := string(content)

	serviceMatch := servicePattern.FindStringSubmatch(text)
	if serviceMatch == nil {
		return "", nil, fmt.Errorf("在 %s 中未找到 service 定义", path)
	}
	serviceName := serviceMatch[1]

	var rpcs []RPCInfo
	for _, m := range rpcPattern.FindAllStringSubmatch(text, -1) {
		rpcs = append(rpcs, RPCInfo{
			MethodName:  m[1],
			RequestType: m[2],
			ReplyType:   m[3],
		})
	}

	return serviceName, rpcs, nil
}

// goModPattern 匹配 go.mod 文件中 module 行.
var goModPattern = regexp.MustCompile(`^module\s+(.+)$`)

// detectModule 从当前目录向上查找 go.mod 获取 module 路径.
func detectModule() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findGoModule(dir)
}

// detectModuleFromProto 从 proto 文件中 go_package 选项提取 module 路径.
func detectModuleFromProto(protoFile string) (string, error) {
	content, err := os.ReadFile(protoFile)
	if err != nil {
		return "", err
	}

	// 尝试从 go_package 选项中提取 module 路径
	goPackagePattern := regexp.MustCompile(`option\s+go_package\s*=\s*"([^"]+)"`)
	if m := goPackagePattern.FindStringSubmatch(string(content)); m != nil {
		pkg := m[1]
		// go_package 格式: "module/api/name/v1;namev1"
		if idx := strings.Index(pkg, ";"); idx != -1 {
			pkg = pkg[:idx]
		}
		// 提取到 /api/ 之前的部分作为 module
		if idx := strings.Index(pkg, "/api/"); idx != -1 {
			return pkg[:idx], nil
		}
		return pkg, nil
	}

	// 回退到 go.mod 查找
	dir := filepath.Dir(protoFile)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return findGoModule(absDir)
}

// findGoModule 从指定目录向上查找 go.mod 获取 module 路径.
func findGoModule(dir string) (string, error) {
	for {
		modPath := filepath.Join(dir, "go.mod")
		f, err := os.Open(modPath)
		if err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if m := goModPattern.FindStringSubmatch(scanner.Text()); m != nil {
					return strings.TrimSpace(m[1]), nil
				}
			}
			return "", fmt.Errorf("在 %s 中未找到 module 行", modPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("未找到 go.mod")
}
