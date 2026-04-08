// Package main 提供 servex CLI 交互式向导.
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Everforest Dark 主题色板.
var (
	efGreen  = lipgloss.Color("#a7c080") // 高亮/选中
	efAqua   = lipgloss.Color("#83c092") // 活跃元素
	efBlue   = lipgloss.Color("#7fbbb3") // 边框/链接
	efYellow = lipgloss.Color("#dbbc7f") // 警告/标题强调
	efOrange = lipgloss.Color("#e69875") // 次要强调
	efRed    = lipgloss.Color("#e67e80") // 错误
	efPurple = lipgloss.Color("#d699b6") // 描述
	efFg     = lipgloss.Color("#d3c6aa") // 前景文字
	efGrey   = lipgloss.Color("#859289") // 灰色/占位符
	efBg     = lipgloss.Color("#2d353b") // 背景
	efBg1    = lipgloss.Color("#343f44") // 次要背景
)

// themeEverforest 创建 Everforest Dark 主题.
func themeEverforest() *huh.Theme {
	t := huh.ThemeBase()

	// Focused 状态
	t.Focused.Title = lipgloss.NewStyle().Foreground(efGreen).Bold(true)
	t.Focused.Description = lipgloss.NewStyle().Foreground(efGrey)
	t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(efRed)
	t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(efRed)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(efAqua).SetString("▸ ")
	t.Focused.MultiSelectSelector = lipgloss.NewStyle().Foreground(efAqua).SetString("▸ ")
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(efGreen)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(efFg)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(efGreen).SetString("[✓] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(efGrey).SetString("[ ] ")
	t.Focused.FocusedButton = lipgloss.NewStyle().Foreground(efBg).Background(efGreen).Bold(true).Padding(0, 1)
	t.Focused.BlurredButton = lipgloss.NewStyle().Foreground(efFg).Background(efBg1).Padding(0, 1)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(efAqua)
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(efGrey)
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(efBlue)

	// Blurred 状态
	t.Blurred.Title = lipgloss.NewStyle().Foreground(efGrey)
	t.Blurred.Description = lipgloss.NewStyle().Foreground(efGrey)
	t.Blurred.SelectedOption = lipgloss.NewStyle().Foreground(efGrey)
	t.Blurred.UnselectedOption = lipgloss.NewStyle().Foreground(efGrey)
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(efGrey)

	return t
}

// shouldRunWizard 检查是否应该启动交互式向导.
// 如果用户显式设置了任何 flag，则跳过向导[CI / 脚本模式].
func shouldRunWizard(cmd *cobra.Command) bool {
	changed := false
	cmd.Flags().Visit(func(_ *pflag.Flag) { changed = true })
	return !changed
}

// runNewWizard 运行 servex new 交互式向导，收集项目配置.
func runNewWizard() (ProjectData, error) {
	var (
		projectName string
		modulePath  string
		mode        string
		transport   []string
		infra       []string
		observe     []string
		auth        []string
		discovery   string
		other       []string
		withWire    bool
	)

	form := huh.NewForm(
		// 项目基本信息
		huh.NewGroup(
			huh.NewInput().
				Title("项目名称").
				Value(&projectName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("项目名称不能为空")
					}
					return nil
				}),
			huh.NewInput().
				Title("Go Module 路径").
				Placeholder("github.com/example/myproject").
				Value(&modulePath),
		).Title("基本信息"),

		// 项目模式
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("项目模式").
				Options(
					huh.NewOption("Monorepo[多服务共享 domain]", "monorepo"),
					huh.NewOption("Standalone[独立单服务]", "standalone"),
				).
				Value(&mode),
		).Title("项目模式"),

		// 传输层
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("传输层").
				Options(
					huh.NewOption("HTTP (REST API)", "http"),
					huh.NewOption("gRPC", "grpc"),
					huh.NewOption("Gateway (HTTP + gRPC 双协议)", "gateway"),
				).
				Value(&transport),
		).Title("传输层"),

		// 基础设施
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("基础设施").
				Options(
					huh.NewOption("MySQL", "mysql"),
					huh.NewOption("PostgreSQL", "postgres"),
					huh.NewOption("SQLite", "sqlite"),
					huh.NewOption("Redis", "redis"),
					huh.NewOption("MongoDB", "mongo"),
					huh.NewOption("Elasticsearch", "es"),
					huh.NewOption("ClickHouse", "clickhouse"),
					huh.NewOption("Kafka", "kafka"),
					huh.NewOption("RabbitMQ", "rabbitmq"),
					huh.NewOption("S3 对象存储", "s3"),
					huh.NewOption("MinIO", "minio"),
					huh.NewOption("Neo4j 图数据库", "neo4j"),
				).
				Value(&infra),
		).Title("基础设施"),

		// 可观测性
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("可观测性").
				Options(
					huh.NewOption("Prometheus Metrics", "metrics"),
					huh.NewOption("OpenTelemetry Tracing", "tracing"),
					huh.NewOption("Profiling 性能剖析", "profiling"),
				).
				Value(&observe),
		).Title("可观测性"),

		// 认证
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("认证").
				Options(
					huh.NewOption("JWT", "jwt"),
					huh.NewOption("RBAC 角色权限", "rbac"),
				).
				Value(&auth),
		).Title("认证"),

		// 服务发现
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("服务发现").
				Options(
					huh.NewOption("不需要", ""),
					huh.NewOption("Consul", "consul"),
					huh.NewOption("etcd", "etcd"),
					huh.NewOption("Nacos", "nacos"),
				).
				Value(&discovery),
		).Title("服务发现"),

		// 其他组件 + Wire
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("其他组件").
				Options(
					huh.NewOption("定时任务 (Scheduler)", "scheduler"),
					huh.NewOption("国际化 (i18n)", "i18n"),
					huh.NewOption("多租户 (Tenant)", "tenant"),
				).
				Value(&other),
			huh.NewConfirm().
				Title("使用 Wire 依赖注入?").
				Value(&withWire),
		).Title("其他"),
	).WithTheme(themeEverforest())

	if err := form.Run(); err != nil {
		return ProjectData{}, err
	}

	// 默认 module 路径
	if modulePath == "" {
		modulePath = "github.com/example/" + projectName
	}

	// 解析传输层选择
	withGRPC := false
	for _, t := range transport {
		if t == "grpc" || t == "gateway" {
			withGRPC = true
			break
		}
	}

	// 构建 ProjectData
	data := ProjectData{
		Name:     projectName,
		Module:   modulePath,
		WithGRPC: withGRPC,
		WithWire: withWire,
		Infra:    parseInfra(strings.Join(infra, ",")),
	}

	// 记录模式供调用方判断
	if mode == "standalone" {
		newStandalone = true
	}

	return data, nil
}

// addServiceWizardResult 保存 add service 向导结果.
type addServiceWizardResult struct {
	Name        string
	Transport   []string
	Infra       []string
	Observe     []string
	Auth        []string
	Discovery   string
	Other       []string
	WithWire    bool
}

// runAddServiceWizard 运行 servex add service 交互式向导.
func runAddServiceWizard() (addServiceWizardResult, error) {
	var result addServiceWizardResult

	form := huh.NewForm(
		// 服务名称
		huh.NewGroup(
			huh.NewInput().
				Title("服务名称").
				Value(&result.Name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("服务名称不能为空")
					}
					return nil
				}),
		).Title("基本信息"),

		// 传输层
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("传输层[HTTP 默认包含]").
				Options(
					huh.NewOption("gRPC", "grpc"),
					huh.NewOption("Gateway (HTTP + gRPC 双协议)", "gateway"),
				).
				Value(&result.Transport),
		).Title("传输层"),

		// 基础设施
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("基础设施").
				Options(
					huh.NewOption("MySQL", "mysql"),
					huh.NewOption("PostgreSQL", "postgres"),
					huh.NewOption("SQLite", "sqlite"),
					huh.NewOption("Redis", "redis"),
					huh.NewOption("MongoDB", "mongo"),
					huh.NewOption("Elasticsearch", "es"),
					huh.NewOption("ClickHouse", "clickhouse"),
					huh.NewOption("Kafka", "kafka"),
					huh.NewOption("RabbitMQ", "rabbitmq"),
					huh.NewOption("S3 对象存储", "s3"),
					huh.NewOption("MinIO", "minio"),
					huh.NewOption("Neo4j 图数据库", "neo4j"),
				).
				Value(&result.Infra),
		).Title("基础设施"),

		// 可观测性
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("可观测性").
				Options(
					huh.NewOption("Prometheus Metrics", "metrics"),
					huh.NewOption("OpenTelemetry Tracing", "tracing"),
					huh.NewOption("Profiling 性能剖析", "profiling"),
				).
				Value(&result.Observe),
		).Title("可观测性"),

		// 认证
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("认证").
				Options(
					huh.NewOption("JWT", "jwt"),
					huh.NewOption("RBAC 角色权限", "rbac"),
				).
				Value(&result.Auth),
		).Title("认证"),

		// 服务发现
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("服务发现").
				Options(
					huh.NewOption("不需要", ""),
					huh.NewOption("Consul", "consul"),
					huh.NewOption("etcd", "etcd"),
					huh.NewOption("Nacos", "nacos"),
				).
				Value(&result.Discovery),
		).Title("服务发现"),

		// 其他组件 + Wire
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("其他组件").
				Options(
					huh.NewOption("定时任务 (Scheduler)", "scheduler"),
					huh.NewOption("国际化 (i18n)", "i18n"),
					huh.NewOption("多租户 (Tenant)", "tenant"),
				).
				Value(&result.Other),
			huh.NewConfirm().
				Title("使用 Wire 依赖注入?").
				Value(&result.WithWire),
		).Title("其他"),
	).WithTheme(themeEverforest())

	if err := form.Run(); err != nil {
		return addServiceWizardResult{}, err
	}

	return result, nil
}

// applyAddServiceWizard 将向导结果应用到 add service 全局 flag 变量.
func applyAddServiceWizard(r addServiceWizardResult) {
	for _, t := range r.Transport {
		switch t {
		case "grpc":
			addWithGRPC = true
		case "gateway":
			addWithGateway = true
		}
	}
	addInfra = strings.Join(r.Infra, ",")
	addObserve = strings.Join(r.Observe, ",")
	addAuth = strings.Join(r.Auth, ",")
	if r.Discovery != "" {
		addDiscovery = r.Discovery
	}
	addOther = strings.Join(r.Other, ",")
	addWithWire = r.WithWire
}
