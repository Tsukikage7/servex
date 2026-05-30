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
	Name        string    // 原始名称 (小写)
	PascalName  string    // PascalCase 名称
	Module      string    // Go module 路径
	RPCs        []RPCInfo // RPC 方法列表
	WithConnect bool      // 是否生成 Connect 注册方法
}

// RPCInfo RPC 方法信息.
type RPCInfo struct {
	MethodName  string // 方法名
	RequestType string // 请求类型
	ReplyType   string // 响应类型
}

// printProtoUsage 输出 proto 命令帮助信息.
func printProtoUsage() {
	fmt.Println(`用法: servex proto <subcommand> [arguments]

子命令:
  add       创建 proto 服务模板 (自动生成 buf 配置)
  client    从 proto 文件使用 buf 生成客户端代码
  server    从 proto 文件生成服务端桩代码
  lint      使用 buf lint 检查 proto 文件规范
  breaking  使用 buf breaking 检测不兼容变更

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

	apiDir := filepath.Join(*output, "api")
	if err := ensureProtoThirdParty(apiDir); err != nil {
		return fmt.Errorf("复制 third_party: %w", err)
	}

	// 自动生成 buf 配置文件 (如不存在)
	bufYamlPath := filepath.Join(apiDir, "buf.yaml")
	if _, err := os.Stat(bufYamlPath); os.IsNotExist(err) {
		if err := generateBufYaml(bufYamlPath); err != nil {
			return fmt.Errorf("生成 buf.yaml: %w", err)
		}
		fmt.Printf("Buf 配置已创建: %s\n", bufYamlPath)
	}

	bufGenYamlPath := filepath.Join(apiDir, "buf.gen.yaml")
	if _, err := os.Stat(bufGenYamlPath); os.IsNotExist(err) {
		if err := generateBufGenYaml(bufGenYamlPath, true); err != nil {
			return fmt.Errorf("生成 buf.gen.yaml: %w", err)
		}
		fmt.Printf("Buf 生成配置已创建: %s\n", bufGenYamlPath)
	}

	return nil
}

func ensureProtoThirdParty(apiDir string) error {
	dst := filepath.Join(apiDir, "third_party")
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return copyEmbedDir(projectTemplates, "templates/project/api/third_party", dst)
}

// runProtoClient 执行 servex proto client 命令，使用 buf generate 生成代码.
func runProtoClient(args []string) error {
	fs := flag.NewFlagSet("proto client", flag.ExitOnError)
	output := fs.String("output", "", "输出目录 (默认: proto 文件所在目录)")
	protocol := fs.String("protocol", "auto", "生成协议: auto|grpc|gateway|connect")
	fs.Usage = func() {
		fmt.Println("用法: servex proto client <proto-file> [options]")
		fmt.Println()
		fmt.Println("从 proto 文件使用 buf 生成 Go 客户端代码.")
		fmt.Println("生成: *.pb.go, *_grpc.pb.go, *_http.pb.go, *_connect.go")
		fmt.Println()
		fmt.Println("前置条件:")
		fmt.Println("  brew install bufbuild/buf/buf")
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

	bufPath, err := exec.LookPath("buf")
	if err != nil {
		return fmt.Errorf("未找到 buf，请先安装:\n\n  macOS:   brew install bufbuild/buf/buf\n  Linux:   sudo apt install -y buf 或 npm install -g @bufbuild/buf\n  手动:    https://buf.build/docs/installation")
	}

	apiDir := findAPIDir(protoFile)

	// 查找或自动生成 buf.gen.yaml。显式协议使用独立配置，避免覆盖用户默认配置。
	genYamlPath, generated, err := resolveBufGenYaml(protoFile, apiDir, *protocol)
	if err != nil {
		return fmt.Errorf("生成 buf.gen.yaml: %w", err)
	}
	if generated {
		fmt.Printf("自动生成 buf.gen.yaml: %s\n", genYamlPath)
	}

	// 确保 buf.yaml 存在
	bufYamlPath := filepath.Join(apiDir, "buf.yaml")
	if _, err := os.Stat(bufYamlPath); os.IsNotExist(err) {
		if err := generateBufYaml(bufYamlPath); err != nil {
			return fmt.Errorf("生成 buf.yaml: %w", err)
		}
		fmt.Printf("自动生成 buf.yaml: %s\n", bufYamlPath)
	}

	// 构建 buf generate 参数
	bufArgs := []string{"generate", "--template", genYamlPath, "--path", protoFile}

	cmd := exec.Command(bufPath, bufArgs...)
	cmd.Dir = apiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buf generate 执行失败: %w", err)
	}

	fmt.Printf("客户端代码已生成: %s\n", outDir)
	return nil
}

func resolveBufGenYaml(protoFile, apiDir, protocol string) (string, bool, error) {
	if protocol != "" && protocol != "auto" {
		genPath := filepath.Join(apiDir, "buf.gen."+protocol+".yaml")
		if err := generateBufGenYamlForProtocol(genPath, protoFile, protocol); err != nil {
			return "", false, err
		}
		return genPath, true, nil
	}

	if genYamlPath := findBufGenYaml(protoFile); genYamlPath != "" {
		return genYamlPath, false, nil
	}

	genPath := filepath.Join(apiDir, "buf.gen.yaml")
	if err := generateBufGenYamlForProtocol(genPath, protoFile, protocol); err != nil {
		return "", false, err
	}
	return genPath, true, nil
}

// runProtoLint 执行 servex proto lint 命令.
func runProtoLint(args []string) error {
	fs := flag.NewFlagSet("proto lint", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("用法: servex proto lint [path]")
		fmt.Println()
		fmt.Println("使用 buf lint 检查 proto 文件规范.")
		fmt.Println("默认 lint 当前目录下的 api/ 目录.")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	bufPath, err := exec.LookPath("buf")
	if err != nil {
		return fmt.Errorf("未找到 buf，请先安装: brew install bufbuild/buf/buf")
	}

	target := "api"
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("目标路径不存在: %s", target)
	}

	cmd := exec.Command(bufPath, "lint", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buf lint 检查发现问题")
	}

	fmt.Println("proto lint 检查通过")
	return nil
}

// runProtoBreaking 执行 servex proto breaking 命令.
func runProtoBreaking(args []string) error {
	fs := flag.NewFlagSet("proto breaking", flag.ExitOnError)
	against := fs.String("against", ".git#branch=main", "对比目标 (默认: main 分支)")
	fs.Usage = func() {
		fmt.Println("用法: servex proto breaking [path] [options]")
		fmt.Println()
		fmt.Println("使用 buf breaking 检测 proto 文件的不兼容变更.")
		fmt.Println("默认对比 main 分支.")
		fmt.Println()
		fmt.Println("选项:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	bufPath, err := exec.LookPath("buf")
	if err != nil {
		return fmt.Errorf("未找到 buf，请先安装: brew install bufbuild/buf/buf")
	}

	target := "api"
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("目标路径不存在: %s", target)
	}

	cmd := exec.Command(bufPath, "breaking", target, "--against", *against)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buf breaking 检测到不兼容变更")
	}

	fmt.Println("proto 兼容性检查通过")
	return nil
}

// findBufGenYaml 从 proto 文件所在目录向上查找 buf.gen.yaml.
func findBufGenYaml(protoFile string) string {
	dir, err := filepath.Abs(filepath.Dir(protoFile))
	if err != nil {
		return ""
	}

	for {
		candidate := filepath.Join(dir, "buf.gen.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// findAPIDir 从 proto 文件路径中找到 api/ 目录.
// 例如 api/order/v1/order.proto → api/
func findAPIDir(protoFile string) string {
	absPath, err := filepath.Abs(protoFile)
	if err != nil {
		return filepath.Dir(protoFile)
	}

	dir := absPath
	for {
		if filepath.Base(dir) == "api" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 回退: 使用 proto 文件所在目录
	return filepath.Dir(protoFile)
}

// generateBufYaml 生成 buf.yaml 配置文件.
func generateBufYaml(path string) error {
	content, err := protoTemplates.ReadFile("templates/proto/buf.yaml.tmpl")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

// generateBufGenYaml 生成 buf.gen.yaml 配置文件.
func generateBufGenYaml(path string, withHTTP bool) error {
	tmplName := "templates/proto/buf.gen.yaml.tmpl"
	if withHTTP {
		tmplName = "templates/proto/buf.gen.http.yaml.tmpl"
	}
	return writeProtoTemplate(path, tmplName)
}

func generateBufGenYamlForProtocol(path, protoFile, protocol string) error {
	switch protocol {
	case "", "auto":
		return generateBufGenYaml(path, hasHTTPAnnotations(protoFile))
	case "grpc":
		return generateBufGenYaml(path, false)
	case "gateway":
		return generateBufGenYaml(path, true)
	case "connect":
		return writeProtoTemplate(path, "templates/proto/buf.gen.connect.yaml.tmpl")
	default:
		return fmt.Errorf("不支持的 proto 生成协议: %s", protocol)
	}
}

func writeProtoTemplate(path, tmplName string) error {
	content, err := protoTemplates.ReadFile(tmplName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
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
	withConnect := fs.Bool("with-connect", false, "生成 Connect 注册方法")
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
		Name:        name,
		PascalName:  serviceName,
		Module:      modulePath,
		RPCs:        rpcs,
		WithConnect: *withConnect,
	}

	// monorepo 模式: 若指定 --service 且检测到 monorepo，输出到 services/<service>/internal/service/
	outDir := *target
	if *service != "" && isMonorepo() {
		outDir = filepath.Join(resolveServiceDirForWrite(*service), "internal", "service")
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
			mod, fErr := readModuleLine(f, modPath)
			f.Close()
			if fErr != nil {
				return "", fErr
			}
			return mod, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("未找到 go.mod")
}

// readModuleLine 从已打开的 go.mod 文件中读取 module 行.
func readModuleLine(f *os.File, modPath string) (string, error) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := goModPattern.FindStringSubmatch(scanner.Text()); m != nil {
			return strings.TrimSpace(m[1]), nil
		}
	}
	return "", fmt.Errorf("在 %s 中未找到 module 行", modPath)
}
