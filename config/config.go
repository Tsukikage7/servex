// Package config 提供配置加载和管理功能.
package config

import (
	"path/filepath"
	"strings"

	"github.com/Tsukikage7/servex/validation"
)

// Validatable 可验证的配置接口.
//
// Deprecated: 请直接使用 validation.Validatable.
type Validatable = validation.Validatable

// GetConfigType 根据文件扩展名获取配置类型.
func GetConfigType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".ini":
		return "ini"
	case ".env":
		return "env"
	case ".properties":
		return "properties"
	default:
		return ""
	}
}
