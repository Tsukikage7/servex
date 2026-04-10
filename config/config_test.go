package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// ConfigTestSuite 配置测试套件.
type ConfigTestSuite struct {
	suite.Suite
	tempDir string
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) SetupSuite() {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "config_test")
	s.Require().NoError(err)
	s.tempDir = tempDir
}

func (s *ConfigTestSuite) TearDownSuite() {
	os.RemoveAll(s.tempDir)
}

// 测试用配置结构.
type TestConfig struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Port    int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

// ValidatableConfig 实现 Validatable 接口的配置.
type ValidatableConfig struct {
	Name string `mapstructure:"name"`
	Port int    `mapstructure:"port"`
}

func (c *ValidatableConfig) Validate() error {
	if c.Name == "" {
		return errors.New("name 不能为空")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("port 必须在 1-65535 之间")
	}
	return nil
}

// InvalidConfig 验证总是失败的配置.
type InvalidConfig struct {
	Value string `mapstructure:"value"`
}

func (c *InvalidConfig) Validate() error {
	return errors.New("验证失败")
}

func (s *ConfigTestSuite) createYAMLFile(name, content string) string {
	path := filepath.Join(s.tempDir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	s.Require().NoError(err)
	return path
}

func (s *ConfigTestSuite) createJSONFile(name, content string) string {
	path := filepath.Join(s.tempDir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	s.Require().NoError(err)
	return path
}

// === Load 测试 ===

func (s *ConfigTestSuite) TestLoad_YAML() {
	content := `
app:
  name: test-app
  version: "1.0.0"
  port: 8080
database:
  host: localhost
  port: 5432
  username: admin
  password: secret
  database: testdb
`
	path := s.createYAMLFile("config.yaml", content)

	config, err := Load[TestConfig](path)
	s.NoError(err)
	s.NotNil(config)
	s.Equal("test-app", config.App.Name)
	s.Equal("1.0.0", config.App.Version)
	s.Equal(8080, config.App.Port)
	s.Equal("localhost", config.Database.Host)
	s.Equal(5432, config.Database.Port)
}

func (s *ConfigTestSuite) TestLoad_JSON() {
	content := `{
  "app": {
    "name": "json-app",
    "version": "2.0.0",
    "port": 9090
  },
  "database": {
    "host": "127.0.0.1",
    "port": 3306
  }
}`
	path := s.createJSONFile("config.json", content)

	config, err := Load[TestConfig](path)
	s.NoError(err)
	s.NotNil(config)
	s.Equal("json-app", config.App.Name)
	s.Equal(9090, config.App.Port)
	s.Equal("127.0.0.1", config.Database.Host)
}

func (s *ConfigTestSuite) TestLoad_FileNotFound() {
	_, err := Load[TestConfig]("/nonexistent/config.yaml")
	s.Error(err)
	s.ErrorIs(err, ErrFileNotFound)
}

func (s *ConfigTestSuite) TestLoad_InvalidYAML() {
	content := `invalid: yaml: content: [}`
	path := s.createYAMLFile("invalid.yaml", content)

	_, err := Load[TestConfig](path)
	s.Error(err)
	s.ErrorIs(err, ErrReadConfig)
}

func (s *ConfigTestSuite) TestLoad_WithDefaults() {
	content := `
app:
  name: my-app
`
	path := s.createYAMLFile("partial.yaml", content)

	defaults := map[string]any{
		"app.port":      3000,
		"database.host": "default-host",
	}

	config, err := Load[TestConfig](path, WithDefaults(defaults))
	s.NoError(err)
	s.Equal("my-app", config.App.Name)
	s.Equal(3000, config.App.Port)
	s.Equal("default-host", config.Database.Host)
}

func (s *ConfigTestSuite) TestLoad_WithValidation_Success() {
	content := `
name: valid-app
port: 8080
`
	path := s.createYAMLFile("valid.yaml", content)

	config, err := Load[ValidatableConfig](path)
	s.NoError(err)
	s.Equal("valid-app", config.Name)
	s.Equal(8080, config.Port)
}

func (s *ConfigTestSuite) TestLoad_WithValidation_Failure() {
	content := `
name: ""
port: 8080
`
	path := s.createYAMLFile("invalid_name.yaml", content)

	_, err := Load[ValidatableConfig](path)
	s.Error(err)
	s.Contains(err.Error(), "配置验证失败")
}

func (s *ConfigTestSuite) TestLoad_WithValidation_InvalidPort() {
	content := `
name: app
port: 99999
`
	path := s.createYAMLFile("invalid_port.yaml", content)

	_, err := Load[ValidatableConfig](path)
	s.Error(err)
	s.Contains(err.Error(), "配置验证失败")
}

func (s *ConfigTestSuite) TestLoad_AlwaysInvalidConfig() {
	content := `value: test`
	path := s.createYAMLFile("always_invalid.yaml", content)

	_, err := Load[InvalidConfig](path)
	s.Error(err)
	s.Contains(err.Error(), "配置验证失败")
}

// === MustLoad 测试 ===

func (s *ConfigTestSuite) TestMustLoad_Success() {
	content := `
app:
  name: must-app
  port: 8080
`
	path := s.createYAMLFile("must.yaml", content)

	s.NotPanics(func() {
		config := MustLoad[TestConfig](path)
		s.Equal("must-app", config.App.Name)
	})
}

func (s *ConfigTestSuite) TestMustLoad_Panic() {
	s.Panics(func() {
		MustLoad[TestConfig]("/nonexistent/file.yaml")
	})
}

// === LoadFromBytes 测试 ===

func (s *ConfigTestSuite) TestLoadFromBytes_YAML() {
	data := []byte(`
app:
  name: bytes-app
  port: 7070
`)

	config, err := LoadFromBytes[TestConfig](data, "yaml")
	s.NoError(err)
	s.Equal("bytes-app", config.App.Name)
	s.Equal(7070, config.App.Port)
}

func (s *ConfigTestSuite) TestLoadFromBytes_JSON() {
	data := []byte(`{"app": {"name": "json-bytes", "port": 6060}}`)

	config, err := LoadFromBytes[TestConfig](data, "json")
	s.NoError(err)
	s.Equal("json-bytes", config.App.Name)
	s.Equal(6060, config.App.Port)
}

func (s *ConfigTestSuite) TestLoadFromBytes_WithDefaults() {
	data := []byte(`app:
  name: partial
`)

	defaults := map[string]any{
		"app.port": 5050,
	}

	config, err := LoadFromBytes[TestConfig](data, "yaml", WithDefaults(defaults))
	s.NoError(err)
	s.Equal("partial", config.App.Name)
	s.Equal(5050, config.App.Port)
}

func (s *ConfigTestSuite) TestLoadFromBytes_WithValidation_Failure() {
	data := []byte(`name: ""
port: 8080`)

	_, err := LoadFromBytes[ValidatableConfig](data, "yaml")
	s.Error(err)
	s.Contains(err.Error(), "配置验证失败")
}

func (s *ConfigTestSuite) TestLoadFromBytes_InvalidFormat() {
	data := []byte(`invalid yaml: [}`)

	_, err := LoadFromBytes[TestConfig](data, "yaml")
	s.Error(err)
}

// === LoadWithSearch 测试 ===

func (s *ConfigTestSuite) TestLoadWithSearch_Found() {
	content := `
app:
  name: search-app
  port: 4040
`
	s.createYAMLFile("app.yaml", content)

	config, err := LoadWithSearch[TestConfig]("app", []string{s.tempDir})
	s.NoError(err)
	s.Equal("search-app", config.App.Name)
}

func (s *ConfigTestSuite) TestLoadWithSearch_NotFound() {
	_, err := LoadWithSearch[TestConfig]("nonexistent", []string{s.tempDir})
	s.Error(err)
}

func (s *ConfigTestSuite) TestLoadWithSearch_MultipleSearchPaths() {
	subDir := filepath.Join(s.tempDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	s.Require().NoError(err)

	content := `app:
  name: subdir-app
`
	err = os.WriteFile(filepath.Join(subDir, "app.yaml"), []byte(content), 0644)
	s.Require().NoError(err)

	config, err := LoadWithSearch[TestConfig]("app", []string{"/nonexistent", subDir})
	s.NoError(err)
	s.Equal("subdir-app", config.App.Name)
}

func (s *ConfigTestSuite) TestLoadWithSearch_WithValidation_Failure() {
	content := `name: ""
port: 8080`
	s.createYAMLFile("validate.yaml", content)

	_, err := LoadWithSearch[ValidatableConfig]("validate", []string{s.tempDir})
	s.Error(err)
	s.Contains(err.Error(), "配置验证失败")
}

// === GetConfigType 测试 ===

func (s *ConfigTestSuite) TestGetConfigType() {
	testCases := []struct {
		filename string
		expected string
	}{
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"config.json", "json"},
		{"config.toml", "toml"},
		{"config.ini", "ini"},
		{".env", "env"},
		{"app.properties", "properties"},
		{"config.unknown", ""},
		{"config", ""},
		{"CONFIG.YAML", "yaml"},
		{"config.JSON", "json"},
	}

	for _, tc := range testCases {
		s.Equal(tc.expected, GetConfigType(tc.filename), "文件: %s", tc.filename)
	}
}

// === Options 测试 ===

func (s *ConfigTestSuite) TestDefaultOptions() {
	opts := DefaultOptions()
	s.NotNil(opts)
	s.NotNil(opts.EnvKeyReplacer)
	s.True(opts.AutomaticEnv)
	s.False(opts.AllowEmptyEnv)
}

func (s *ConfigTestSuite) TestWithEnvPrefix() {
	content := `
app:
  name: env-app
  port: 8080
`
	path := s.createYAMLFile("env_prefix.yaml", content)

	// 设置环境变量
	os.Setenv("MYAPP_APP_NAME", "overridden-name")
	defer os.Unsetenv("MYAPP_APP_NAME")

	config, err := Load[TestConfig](path, WithEnvPrefix("MYAPP"))
	s.NoError(err)
	// 注意：viper 的环境变量绑定需要显式 BindEnv，这里主要测试选项设置
	s.NotNil(config)
}

func (s *ConfigTestSuite) TestWithAutomaticEnv() {
	content := `app:
  name: auto-env-app
`
	path := s.createYAMLFile("auto_env.yaml", content)

	config, err := Load[TestConfig](path, WithAutomaticEnv())
	s.NoError(err)
	s.NotNil(config)
}

func (s *ConfigTestSuite) TestWithConfigType() {
	// 创建一个没有扩展名的文件
	content := `app:
  name: no-ext-app
  port: 1234
`
	path := filepath.Join(s.tempDir, "config_noext")
	err := os.WriteFile(path, []byte(content), 0644)
	s.Require().NoError(err)

	config, err := Load[TestConfig](path, WithConfigType("yaml"))
	s.NoError(err)
	s.Equal("no-ext-app", config.App.Name)
	s.Equal(1234, config.App.Port)
}

// === 错误常量测试 ===

func (s *ConfigTestSuite) TestErrors() {
	s.Equal("配置为空", ErrNilConfig.Error())
	s.Equal("配置文件不存在", ErrFileNotFound.Error())
	s.Equal("不支持的配置文件类型", ErrInvalidType.Error())
}

// === ConfigFieldError 测试 ===

func (s *ConfigTestSuite) TestConfigFieldError_Error() {
	err := &ConfigFieldError{
		Field:   "database.host",
		Source:  "file:config.yaml",
		Message: "类型不匹配",
	}
	s.Contains(err.Error(), "database.host")
	s.Contains(err.Error(), "file:config.yaml")
	s.Contains(err.Error(), "类型不匹配")
}

func (s *ConfigTestSuite) TestConfigFieldError_ErrorWithExpectedActual() {
	err := &ConfigFieldError{
		Field:    "server.port",
		Source:   "env:SERVER_PORT",
		Message:  "类型不匹配",
		Expected: "int",
		Actual:   "\"abc\"",
	}
	s.Contains(err.Error(), "期望: int")
	s.Contains(err.Error(), "实际: \"abc\"")
}

func (s *ConfigTestSuite) TestConfigFieldError_Unwrap_Default() {
	err := &ConfigFieldError{
		Field:   "test",
		Source:  "test",
		Message: "test",
	}
	s.ErrorIs(err, ErrValidation)
}

func (s *ConfigTestSuite) TestConfigFieldError_Unwrap_Custom() {
	err := &ConfigFieldError{
		Field:   "test",
		Source:  "test",
		Message: "test",
		Err:     ErrUnmarshal,
	}
	s.ErrorIs(err, ErrUnmarshal)
}

func (s *ConfigTestSuite) TestNewFieldError() {
	err := NewFieldError("db.port", "file:app.yaml", "字段缺失")
	s.Equal("db.port", err.Field)
	s.Equal("file:app.yaml", err.Source)
	s.Equal("字段缺失", err.Message)
}

func (s *ConfigTestSuite) TestNewFieldTypeError() {
	err := NewFieldTypeError("db.port", "env:DB_PORT", "int", "\"hello\"")
	s.Equal("db.port", err.Field)
	s.Equal("env:DB_PORT", err.Source)
	s.Equal("类型不匹配", err.Message)
	s.Equal("int", err.Expected)
	s.Equal("\"hello\"", err.Actual)
}

func (s *ConfigTestSuite) TestLoad_ValidationError_IsConfigFieldError() {
	content := `name: ""
port: 8080`
	path := s.createYAMLFile("field_error_test.yaml", content)

	_, err := Load[ValidatableConfig](path)
	s.Error(err)

	var fieldErr *ConfigFieldError
	s.ErrorAs(err, &fieldErr)
	s.Contains(fieldErr.Source, "file:")
	s.ErrorIs(err, ErrValidation)
}
