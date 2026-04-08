package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

// ClientData 外部服务适配器模板数据.
type ClientData struct {
	TargetName      string // 目标服务 PascalCase (User)
	TargetNameLower string // 目标服务 camelCase (user)
	CallerName      string // 调用方服务名 (order)
	Module          string // Go module 路径
}

// runGenClient 执行 servex gen client 命令.
func runGenClient(targetName, callerName, module, outputDir string) error {
	if targetName == "" {
		return fmt.Errorf("target service name is required")
	}
	if callerName == "" {
		return fmt.Errorf("caller service name (--service) is required")
	}

	if module == "" {
		m, err := detectModule()
		if err != nil {
			return fmt.Errorf("请指定 --module 或在 go.mod 目录下运行: %w", err)
		}
		module = m
	}

	data := ClientData{
		TargetName:      toPascalCase(targetName),
		TargetNameLower: strings.ToLower(toPascalCase(targetName)),
		CallerName:      strings.ToLower(toPascalCase(callerName)),
		Module:          module,
	}

	return generateClient(data, outputDir)
}

// generateClient 根据模板数据生成外部服务适配器文件.
func generateClient(data ClientData, outputDir string) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
	}

	// 1. domain/<caller>/ports.go — Anti-corruption layer interface
	portsPath := filepath.Join(outputDir, "domain", data.CallerName, "ports.go")
	if err := renderTemplate(aggregateTemplates, "templates/aggregate/ports.go.tmpl", portsPath, data, funcMap); err != nil {
		return fmt.Errorf("render ports.go: %w", err)
	}

	// 2. services/<caller>-service/internal/adapter/external/<target>_client.go
	clientPath := filepath.Join(outputDir, "services", data.CallerName+"-service",
		"internal", "adapter", "external", toSnakeCase(data.TargetName)+"_client.go")
	if err := renderTemplate(aggregateTemplates, "templates/aggregate/adapter_external.go.tmpl", clientPath, data, funcMap); err != nil {
		return fmt.Errorf("render adapter_external: %w", err)
	}

	fmt.Printf("Client adapter for %q generated (caller: %s):\n", data.TargetName, data.CallerName)
	fmt.Printf("  domain/%s/ports.go (anti-corruption layer)\n", data.CallerName)
	fmt.Printf("  services/%s-service/internal/adapter/external/%s_client.go\n",
		data.CallerName, toSnakeCase(data.TargetName))

	return nil
}
