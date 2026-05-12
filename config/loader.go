package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Tsukikage7/servex/v2/validation"
	"github.com/spf13/viper"
)

// Load 从文件加载配置.
// 支持 yaml, json, toml 等格式（根据文件扩展名自动识别）.
// 如果配置类型实现了 Validatable 接口，会自动进行验证.
func Load[T any](configPath string, opts ...Option) (*T, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// 检查文件是否存在
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return nil, ErrFileNotFound
	}

	v := viper.New()

	// 设置配置文件
	v.SetConfigFile(configPath)

	// 显式指定配置类型
	if options.ConfigType != "" {
		v.SetConfigType(options.ConfigType)
	}

	// 应用通用选项
	applyOptions(v, options)

	// 读取配置文件
	source := "file:" + configPath
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadConfig, err)
	}

	// 解析并验证
	return unmarshalAndValidate[T](v, source)
}

// MustLoad 加载配置，失败时 panic.
func MustLoad[T any](configPath string, opts ...Option) *T {
	config, err := Load[T](configPath, opts...)
	if err != nil {
		panic(err)
	}
	return config
}

// LoadFromBytes 从字节数组加载配置.
func LoadFromBytes[T any](data []byte, configType string, opts ...Option) (*T, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	v := viper.New()
	v.SetConfigType(configType)

	// 应用通用选项
	applyOptions(v, options)

	// 从字节读取
	source := "bytes:" + configType
	if err := v.ReadConfig(strings.NewReader(string(data))); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadConfig, err)
	}

	// 解析并验证
	return unmarshalAndValidate[T](v, source)
}

// LoadWithSearch 在多个目录中搜索配置文件.
func LoadWithSearch[T any](configName string, searchPaths []string, opts ...Option) (*T, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	v := viper.New()
	v.SetConfigName(configName)

	// 添加搜索路径
	for _, path := range searchPaths {
		v.AddConfigPath(path)
	}

	// 应用通用选项
	applyOptions(v, options)

	// 读取配置
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadConfig, err)
	}

	// 解析并验证
	source := "search:" + configName
	return unmarshalAndValidate[T](v, source)
}

// applyOptions 应用通用选项到 viper 实例.
func applyOptions(v *viper.Viper, options *Options) {
	// 设置默认值
	for key, value := range options.Defaults {
		v.SetDefault(key, value)
	}

	// 环境变量支持
	if options.EnvPrefix != "" {
		v.SetEnvPrefix(options.EnvPrefix)
	}

	if options.EnvKeyReplacer != nil {
		v.SetEnvKeyReplacer(options.EnvKeyReplacer)
	}

	if options.AutomaticEnv {
		v.AutomaticEnv()
	}

	v.AllowEmptyEnv(options.AllowEmptyEnv)
}

// unmarshalAndValidate 解析配置并验证.
func unmarshalAndValidate[T any](v *viper.Viper, source string) (*T, error) {
	config := new(T)
	if err := v.Unmarshal(config); err != nil {
		return nil, &ConfigFieldError{
			Source:  source,
			Message: fmt.Sprintf("%v: %v", ErrUnmarshal, err),
			Err:     ErrUnmarshal,
		}
	}

	// 如果实现了 Validatable 接口，进行验证
	if validator, ok := any(config).(validation.Validatable); ok {
		if err := validator.Validate(); err != nil {
			return nil, &ConfigFieldError{
				Source:  source,
				Message: fmt.Sprintf("%v: %v", ErrValidation, err),
				Err:     ErrValidation,
			}
		}
	}

	return config, nil
}
