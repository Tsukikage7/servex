// Package main 提供 servex CLI 脚手架工具入口.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd servex 根命令.
var rootCmd = &cobra.Command{
	Use:   "servex",
	Short: "servex — Go 微服务开发工具包 CLI",
	Long:  "servex 是一个 Go 微服务开发工具包的 CLI 脚手架工具，提供项目创建、DDD 代码生成、Proto 管理等功能.",
}

// --- new 命令 ---

var (
	newModule     string
	newStandalone bool
	newWithGRPC   bool
	newWithDB     bool
	newWithRedis  bool
	newWithWire   bool
)

// newCmd 项目创建命令（默认 monorepo 模式）.
var newCmd = &cobra.Command{
	Use:   "new <project-name>",
	Short: "创建新的 servex 项目（默认 monorepo 模式）",
	Long:  "从模板创建 servex 微服务项目。默认创建 monorepo 结构，使用 --standalone 创建独立单服务项目.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if newModule == "" {
			newModule = "github.com/example/" + args[0]
		}
		data := ProjectData{
			Name:      args[0],
			Module:    newModule,
			WithGRPC:  newWithGRPC,
			WithDB:    newWithDB,
			WithRedis: newWithRedis,
			WithWire:  newWithWire,
		}
		if newStandalone {
			return generateProject(data)
		}
		return generateMonorepo(data)
	},
}

// --- add 命令组 ---

var (
	addWithGRPC  bool
	addWithDB    bool
	addWithRedis bool
	addWithWire  bool
)

// addCmd 添加组件父命令.
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "添加组件（service）",
}

// addServiceCmd 微服务添加命令.
var addServiceCmd = &cobra.Command{
	Use:   "service <name>",
	Short: "在 monorepo 中添加微服务",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAddService(args)
	},
}

// --- gen 命令组 ---

// genCmd 代码生成父命令.
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "生成代码（aggregate/client/entity/valueobject/dockerfile/justfile）",
}

var (
	genAggFields   string
	genAggOutput   string
	genAggModule   string
	genAggService  string
	genAggCommands string
	genAggUnique   string
)

// genAggregateCmd DDD 聚合生成命令.
var genAggregateCmd = &cobra.Command{
	Use:   "aggregate <name>",
	Short: "生成 DDD 聚合代码（aggregate/event/repository/command/query/service）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		module := genAggModule
		if module == "" {
			m, err := detectModule()
			if err != nil {
				return fmt.Errorf("请指定 --module 或在 go.mod 目录下运行: %w", err)
			}
			module = m
		}
		fields := parseFields(genAggFields)
		data := buildAggregateData(args[0], module, fields, genAggCommands, genAggUnique)
		data.Service = genAggService
		return generateAggregate(data, genAggOutput)
	},
}

var (
	genDockerName string
	genDockerPort string
	genDockerOut  string
)

// genDockerfileCmd Dockerfile 生成命令.
var genDockerfileCmd = &cobra.Command{
	Use:   "dockerfile",
	Short: "生成多阶段 Dockerfile",
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateDockerfile(DockerData{Name: genDockerName, Port: genDockerPort}, genDockerOut)
	},
}

var (
	genJustName   string
	genJustModule string
	genJustOut    string
)

// genJustfileCmd justfile 生成命令.
var genJustfileCmd = &cobra.Command{
	Use:   "justfile",
	Short: "生成 justfile（build/test/lint/proto/wire/docker）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if genJustModule == "" {
			genJustModule = "github.com/example/" + genJustName
		}
		return generateJustfile(JustfileData{Name: genJustName, Module: genJustModule}, genJustOut)
	},
}

// --- gen client/entity/valueobject 命令 ---

var (
	genClientService string
	genClientModule  string
	genClientOutput  string
)

// genClientCmd 外部服务客户端适配器生成命令.
var genClientCmd = &cobra.Command{
	Use:   "client <target>",
	Short: "生成外部服务客户端适配器（防腐层 + gRPC client）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenClient(args[0], genClientService, genClientModule, genClientOutput)
	},
}

var (
	genEntityAggregate string
	genEntityFields    string
	genEntityModule    string
	genEntityOutput    string
)

// genEntityCmd 子实体生成命令.
var genEntityCmd = &cobra.Command{
	Use:   "entity <name>",
	Short: "生成 DDD 子实体（有 ID，可变，属于聚合）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenEntity(args[0], genEntityAggregate, genEntityFields, genEntityModule, genEntityOutput)
	},
}

var (
	genVOAggregate string
	genVOFields    string
	genVOModule    string
	genVOOutput    string
)

// genValueObjectCmd 值对象生成命令.
var genValueObjectCmd = &cobra.Command{
	Use:   "valueobject <name>",
	Short: "生成 DDD 值对象（无 ID，不可变，属于聚合）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenValueObject(args[0], genVOAggregate, genVOFields, genVOModule, genVOOutput)
	},
}

// --- proto 命令组 ---

// protoCmd Proto 管理父命令.
var protoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Proto 文件管理（add/client/server）",
}

var (
	protoAddModule string
	protoAddOutput string
)

// protoAddCmd Proto 模板创建命令.
var protoAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "创建 proto 服务模板文件",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProtoAdd(buildProtoAddArgs(args[0], protoAddModule, protoAddOutput))
	},
}

var protoClientOutput string

// protoClientCmd Proto 客户端代码生成命令.
var protoClientCmd = &cobra.Command{
	Use:   "client <proto-file>",
	Short: "从 proto 文件生成客户端代码（pb.go/grpc.go/http.go）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProtoClient(buildProtoClientArgs(args[0], protoClientOutput))
	},
}

var (
	protoServerTarget  string
	protoServerService string
)

// protoServerCmd Proto 服务端桩代码生成命令.
var protoServerCmd = &cobra.Command{
	Use:   "server <proto-file>",
	Short: "从 proto 文件生成服务端实现桩代码",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProtoServer(buildProtoServerArgs(args[0], protoServerTarget, protoServerService))
	},
}

// --- run 命令 ---

var (
	runEntry string
	runRace  bool
)

// runCmd 运行服务命令.
var runCmdDef = &cobra.Command{
	Use:   "run [-- args...]",
	Short: "运行当前项目（自动检测入口）",
	Long: `运行当前项目，自动检测 main 包入口:
  1. ./cmd/server   (优先)
  2. ./cmd/<dirname>
  3. .              (回退)`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRun(args)
	},
}

// --- upgrade 命令 ---

// upgradeCmd 自升级命令.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "升级 servex CLI 到最新版本",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpgrade(args)
	},
}

// --- version 命令 ---

// versionCmd 版本信息命令.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		runVersion()
	},
}

// buildProtoAddArgs 构建 proto add 参数列表.
func buildProtoAddArgs(name, module, output string) []string {
	args := []string{name}
	if module != "" {
		args = append(args, "-module", module)
	}
	if output != "." {
		args = append(args, "-output", output)
	}
	return args
}

// buildProtoClientArgs 构建 proto client 参数列表.
func buildProtoClientArgs(protoFile, output string) []string {
	args := []string{protoFile}
	if output != "" {
		args = append(args, "-output", output)
	}
	return args
}

// buildProtoServerArgs 构建 proto server 参数列表.
func buildProtoServerArgs(protoFile, target, service string) []string {
	args := []string{protoFile}
	if target != "internal/service" {
		args = append(args, "-target", target)
	}
	if service != "" {
		args = append(args, "-service", service)
	}
	return args
}

func init() {
	// new
	newCmd.Flags().StringVar(&newModule, "module", "", "Go module 路径 (默认: github.com/example/<project>)")
	newCmd.Flags().BoolVar(&newStandalone, "standalone", false, "创建独立单服务项目（默认: monorepo 模式）")
	newCmd.Flags().BoolVar(&newWithGRPC, "with-grpc", false, "包含 gRPC 服务端")
	newCmd.Flags().BoolVar(&newWithDB, "with-db", false, "包含数据库 (GORM)")
	newCmd.Flags().BoolVar(&newWithRedis, "with-redis", false, "包含 Redis")
	newCmd.Flags().BoolVar(&newWithWire, "with-wire", false, "包含 Wire 依赖注入")

	// add service
	addServiceCmd.Flags().BoolVar(&addWithGRPC, "with-grpc", false, "包含 gRPC 服务端")
	addServiceCmd.Flags().BoolVar(&addWithDB, "with-db", false, "包含数据库 (GORM)")
	addServiceCmd.Flags().BoolVar(&addWithRedis, "with-redis", false, "包含 Redis")
	addServiceCmd.Flags().BoolVar(&addWithWire, "with-wire", false, "包含 Wire 依赖注入")

	// add 子命令
	addCmd.AddCommand(addServiceCmd)

	// gen aggregate
	genAggregateCmd.Flags().StringVar(&genAggFields, "fields", "", `字段定义 (如 "id:uint64,name:string,email:string")`)
	genAggregateCmd.Flags().StringVar(&genAggOutput, "output", ".", "输出目录")
	genAggregateCmd.Flags().StringVar(&genAggModule, "module", "", "Go module 路径 (默认: 从 go.mod 读取)")
	genAggregateCmd.Flags().StringVar(&genAggService, "service", "", "目标服务名，生成 adapter/persistence 层到对应服务")
	genAggregateCmd.Flags().StringVar(&genAggCommands, "commands", "", `业务命令 (如 "Place,Cancel,Ship")`)
	genAggregateCmd.Flags().StringVar(&genAggUnique, "unique", "", `唯一字段，生成 FindByXxx (如 "email,username")`)

	// gen dockerfile
	genDockerfileCmd.Flags().StringVar(&genDockerName, "name", "server", "服务名称")
	genDockerfileCmd.Flags().StringVar(&genDockerPort, "port", "8080", "暴露端口")
	genDockerfileCmd.Flags().StringVar(&genDockerOut, "output", ".", "输出目录")

	// gen justfile
	genJustfileCmd.Flags().StringVar(&genJustName, "name", "server", "服务名称")
	genJustfileCmd.Flags().StringVar(&genJustModule, "module", "", "Go module 路径")
	genJustfileCmd.Flags().StringVar(&genJustOut, "output", ".", "输出目录")

	// gen 子命令
	genCmd.AddCommand(genAggregateCmd, genDockerfileCmd, genJustfileCmd, genClientCmd, genEntityCmd, genValueObjectCmd)

	// gen client
	genClientCmd.Flags().StringVar(&genClientService, "service", "", "调用方服务名（必填）")
	genClientCmd.Flags().StringVar(&genClientModule, "module", "", "Go module 路径 (默认: 从 go.mod 读取)")
	genClientCmd.Flags().StringVar(&genClientOutput, "output", ".", "输出目录")

	// gen entity
	genEntityCmd.Flags().StringVar(&genEntityAggregate, "aggregate", "", "所属聚合名（必填）")
	genEntityCmd.Flags().StringVar(&genEntityFields, "fields", "", `字段定义 (如 "id:uint64,name:string")`)
	genEntityCmd.Flags().StringVar(&genEntityModule, "module", "", "Go module 路径")
	genEntityCmd.Flags().StringVar(&genEntityOutput, "output", ".", "输出目录")

	// gen valueobject
	genValueObjectCmd.Flags().StringVar(&genVOAggregate, "aggregate", "", "所属聚合名（必填）")
	genValueObjectCmd.Flags().StringVar(&genVOFields, "fields", "", `字段定义 (如 "street:string,city:string")`)
	genValueObjectCmd.Flags().StringVar(&genVOModule, "module", "", "Go module 路径")
	genValueObjectCmd.Flags().StringVar(&genVOOutput, "output", ".", "输出目录")

	// proto add
	protoAddCmd.Flags().StringVar(&protoAddModule, "module", "", "Go module 路径 (默认: 从 go.mod 读取)")
	protoAddCmd.Flags().StringVar(&protoAddOutput, "output", ".", "输出目录")

	// proto client
	protoClientCmd.Flags().StringVar(&protoClientOutput, "output", "", "输出目录 (默认: proto 文件所在目录)")

	// proto server
	protoServerCmd.Flags().StringVar(&protoServerTarget, "target", "internal/service", "输出目录")
	protoServerCmd.Flags().StringVar(&protoServerService, "service", "", "目标服务名（monorepo 模式）")

	// proto 子命令
	protoCmd.AddCommand(protoAddCmd, protoClientCmd, protoServerCmd)

	// 注册所有顶级命令
	rootCmd.AddCommand(newCmd, addCmd, genCmd, protoCmd, runCmdDef, upgradeCmd, versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
