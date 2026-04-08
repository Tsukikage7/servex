package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

// EntityData 子实体模板数据.
type EntityData struct {
	Name        string  // PascalCase 名称
	NameLower   string  // camelCase 名称
	Aggregate   string  // 所属聚合名 (PascalCase)
	AggLower    string  // 所属聚合名 (camelCase, 包名)
	Fields      []Field // 所有字段（含 ID）
	NonIDFields []Field // 非 ID 字段
	IDType      string  // ID 字段类型
	NeedsTime   bool
}

// ValueObjectData 值对象模板数据.
type ValueObjectData struct {
	Name      string  // PascalCase 名称
	NameLower string  // camelCase 名称
	Aggregate string  // 所属聚合名 (PascalCase)
	AggLower  string  // 所属聚合名 (camelCase, 包名)
	Fields    []Field // 所有字段
	NeedsTime bool
}

// runGenEntity 执行 servex gen entity 命令.
func runGenEntity(name, aggregate, fieldsStr, module, outputDir string) error {
	if name == "" {
		return fmt.Errorf("entity name is required")
	}
	if aggregate == "" {
		return fmt.Errorf("aggregate name (--aggregate) is required")
	}

	fields := parseFields(fieldsStr)
	idType := "uint64"
	var nonIDFields []Field

	for _, f := range fields {
		if strings.EqualFold(f.Name, "Id") || strings.EqualFold(f.Name, "ID") {
			idType = f.Type
		} else {
			nonIDFields = append(nonIDFields, f)
		}
	}

	data := EntityData{
		Name:        toPascalCase(name),
		NameLower:   strings.ToLower(toPascalCase(name)),
		Aggregate:   toPascalCase(aggregate),
		AggLower:    strings.ToLower(toPascalCase(aggregate)),
		Fields:      fields,
		NonIDFields: nonIDFields,
		IDType:      idType,
		NeedsTime:   needsTimeImport(fields),
	}

	return generateEntity(data, outputDir)
}

// generateEntity 根据模板数据生成子实体文件.
func generateEntity(data EntityData, outputDir string) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
	}

	outPath := filepath.Join(outputDir, "domain", data.AggLower, toSnakeCase(data.Name)+".go")
	if err := renderTemplate(aggregateTemplates, "templates/aggregate/entity.go.tmpl", outPath, data, funcMap); err != nil {
		return fmt.Errorf("render entity: %w", err)
	}

	fmt.Printf("Entity %q generated in aggregate %q:\n", data.Name, data.Aggregate)
	fmt.Printf("  domain/%s/%s.go\n", data.AggLower, toSnakeCase(data.Name))

	return nil
}

// runGenValueObject 执行 servex gen valueobject 命令.
func runGenValueObject(name, aggregate, fieldsStr, module, outputDir string) error {
	if name == "" {
		return fmt.Errorf("value object name is required")
	}
	if aggregate == "" {
		return fmt.Errorf("aggregate name (--aggregate) is required")
	}

	fields := parseFields(fieldsStr)

	data := ValueObjectData{
		Name:      toPascalCase(name),
		NameLower: strings.ToLower(toPascalCase(name)),
		Aggregate: toPascalCase(aggregate),
		AggLower:  strings.ToLower(toPascalCase(aggregate)),
		Fields:    fields,
		NeedsTime: needsTimeImport(fields),
	}

	return generateValueObject(data, outputDir)
}

// generateValueObject 根据模板数据生成值对象文件.
func generateValueObject(data ValueObjectData, outputDir string) error {
	funcMap := template.FuncMap{
		"toPascalCase": toPascalCase,
		"toCamelCase":  toCamelCase,
		"toSnakeCase":  toSnakeCase,
	}

	outPath := filepath.Join(outputDir, "domain", data.AggLower, toSnakeCase(data.Name)+".go")
	if err := renderTemplate(aggregateTemplates, "templates/aggregate/valueobject.go.tmpl", outPath, data, funcMap); err != nil {
		return fmt.Errorf("render valueobject: %w", err)
	}

	fmt.Printf("Value object %q generated in aggregate %q:\n", data.Name, data.Aggregate)
	fmt.Printf("  domain/%s/%s.go\n", data.AggLower, toSnakeCase(data.Name))

	return nil
}
