package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ConfigTestSuite 配置测试套件.
type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) TestValidate_NilConfig() {
	var config *Config
	err := config.Validate()

	s.Error(err)
	s.IsType(&ConfigError{}, err)
	s.Equal("config", err.(*ConfigError).Field)
}

func (s *ConfigTestSuite) TestValidate_EmptyConfig() {
	config := &Config{}
	err := config.Validate()

	s.NoError(err)
}

func (s *ConfigTestSuite) TestValidate_ValidFullConfig() {
	config := &Config{
		Type:   TypeZap,
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: OutputConsole,
	}

	s.NoError(config.Validate())
}

func (s *ConfigTestSuite) TestValidate_InvalidLevel() {
	config := &Config{Level: "invalid_level"}
	err := config.Validate()

	s.Error(err)
	s.Equal("level", err.(*ConfigError).Field)
}

func (s *ConfigTestSuite) TestValidate_InvalidFormat() {
	config := &Config{Format: "xml"}
	err := config.Validate()

	s.Error(err)
	s.Equal("format", err.(*ConfigError).Field)
}

func (s *ConfigTestSuite) TestValidate_InvalidOutput() {
	config := &Config{Output: "database"}
	err := config.Validate()

	s.Error(err)
	s.Equal("output", err.(*ConfigError).Field)
}

func (s *ConfigTestSuite) TestValidate_FileOutputRequiresLogDir() {
	config := &Config{Output: OutputFile, LogDir: ""}
	err := config.Validate()

	s.Error(err)
	s.Equal("log_dir", err.(*ConfigError).Field)
}

func (s *ConfigTestSuite) TestValidate_BothOutputRequiresLogDir() {
	config := &Config{Output: OutputBoth, LogDir: ""}
	err := config.Validate()

	s.Error(err)
	s.Equal("log_dir", err.(*ConfigError).Field)
}

func (s *ConfigTestSuite) TestValidate_FileOutputWithLogDir() {
	config := &Config{Output: OutputFile, LogDir: "/tmp/logs"}

	s.NoError(config.Validate())
}

func (s *ConfigTestSuite) TestValidate_ConsoleOutputWithoutLogDir() {
	config := &Config{Output: OutputConsole}

	s.NoError(config.Validate())
}

func (s *ConfigTestSuite) TestApplyDefaults() {
	config := &Config{}
	config.ApplyDefaults()

	s.Equal(TypeZap, config.Type)
	s.Equal(LevelInfo, config.Level)
	s.Equal(FormatConsole, config.Format)
	s.Equal(OutputConsole, config.Output)
	s.Equal("service", config.ServiceName)
	s.Equal("timestamp", config.TimeKey)
	s.Equal("level", config.LevelKey)
	s.Equal("msg", config.MessageKey)
	s.Equal("caller", config.CallerKey)
	s.Equal(TimeFormatDateTime, config.TimeFormat)
	s.Equal(EncodeLevelCapital, config.EncodeLevel)
	s.Equal(EncodeCallerShort, config.EncodeCaller)
	s.Equal(RotationDaily, config.RotationTime)
	s.Equal(7, config.MaxAge)
}

func (s *ConfigTestSuite) TestApplyDefaults_PreservesExistingValues() {
	config := &Config{
		Type:        TypeZap,
		Level:       LevelDebug,
		Format:      FormatConsole,
		Output:      OutputFile,
		ServiceName: "my-service",
		LogDir:      "/var/log",
		MaxAge:      30,
	}
	config.ApplyDefaults()

	s.Equal(LevelDebug, config.Level)
	s.Equal(FormatConsole, config.Format)
	s.Equal("my-service", config.ServiceName)
	s.Equal(30, config.MaxAge)
}

func (s *ConfigTestSuite) TestNeedsFileOutput() {
	testCases := []struct {
		output string
		want   bool
	}{
		{OutputConsole, false},
		{OutputFile, true},
		{OutputBoth, true},
		{"FILE", true},
		{"BOTH", true},
		{"Console", false},
	}

	for _, tc := range testCases {
		config := &Config{Output: tc.output}
		s.Equal(tc.want, config.needsFileOutput(), "output: %s", tc.output)
	}
}

func (s *ConfigTestSuite) TestShouldOutputToConsole() {
	testCases := []struct {
		name           string
		output         string
		consoleEnabled bool
		want           bool
	}{
		{"console output", OutputConsole, false, true},
		{"file output", OutputFile, false, false},
		{"both output", OutputBoth, false, true},
		{"file with console enabled", OutputFile, true, true},
	}

	for _, tc := range testCases {
		config := &Config{Output: tc.output, ConsoleEnabled: tc.consoleEnabled}
		s.Equal(tc.want, config.shouldOutputToConsole(), tc.name)
	}
}

func (s *ConfigTestSuite) TestDefaultConfig() {
	config := DefaultConfig()

	s.NotNil(config)
	s.Equal(TypeZap, config.Type)
	s.Equal(LevelInfo, config.Level)
}

func (s *ConfigTestSuite) TestNewDevConfig() {
	config := NewDevConfig()

	s.NotNil(config)
	s.Equal(LevelDebug, config.Level)
	s.Equal(FormatConsole, config.Format)
	s.Equal(OutputConsole, config.Output)
	s.True(config.EnableCaller)
	s.Equal(EncodeLevelCapitalColor, config.EncodeLevel)
}

func (s *ConfigTestSuite) TestNewProdConfig() {
	config := NewProdConfig("test-service", "/var/log/app")

	s.NotNil(config)
	s.Equal("test-service", config.ServiceName)
	s.Equal("/var/log/app", config.LogDir)
	s.Equal(LevelInfo, config.Level)
	s.Equal(FormatJSON, config.Format)
	s.Equal(OutputBoth, config.Output)
	s.True(config.RotationEnabled)
	s.Equal(RotationDaily, config.RotationTime)
	s.Equal(30, config.MaxAge)
	s.True(config.Compress)
	s.True(config.EnableCaller)
	s.True(config.EnableStacktrace)
}

func (s *ConfigTestSuite) TestConfigError() {
	err := &ConfigError{Field: "level", Message: "invalid value"}
	expected := "logger config error [level]: invalid value"

	s.Equal(expected, err.Error())
}

// ValidatorTestSuite 验证函数测试套件.
type ValidatorTestSuite struct {
	suite.Suite
}

func TestValidatorSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

func (s *ValidatorTestSuite) TestIsValidLevel() {
	validLevels := []string{
		"debug", "DEBUG", "Debug",
		"info", "INFO", "Info",
		"warn", "WARN", "warning", "WARNING",
		"error", "ERROR",
		"fatal", "FATAL",
		"panic", "PANIC",
	}

	for _, level := range validLevels {
		s.True(isValidLevel(level), "level: %s", level)
	}

	invalidLevels := []string{"trace", "verbose", "invalid", ""}
	for _, level := range invalidLevels {
		s.False(isValidLevel(level), "level: %s", level)
	}
}

func (s *ValidatorTestSuite) TestIsValidFormat() {
	validFormats := []string{"json", "JSON", "console", "CONSOLE"}
	for _, format := range validFormats {
		s.True(isValidFormat(format), "format: %s", format)
	}

	invalidFormats := []string{"xml", "text", ""}
	for _, format := range invalidFormats {
		s.False(isValidFormat(format), "format: %s", format)
	}
}

func (s *ValidatorTestSuite) TestIsValidOutput() {
	validOutputs := []string{"console", "CONSOLE", "file", "FILE", "both", "BOTH"}
	for _, output := range validOutputs {
		s.True(isValidOutput(output), "output: %s", output)
	}

	invalidOutputs := []string{"database", "network", ""}
	for _, output := range invalidOutputs {
		s.False(isValidOutput(output), "output: %s", output)
	}
}

// 保留 assert 风格的表驱动测试示例
func TestConfig_Validate_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		wantErr  bool
		errField string
	}{
		{
			name:     "nil config",
			config:   nil,
			wantErr:  true,
			errField: "config",
		},
		{
			name:    "empty config is valid",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "valid full config",
			config: &Config{
				Type:   TypeZap,
				Level:  LevelInfo,
				Format: FormatJSON,
				Output: OutputConsole,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errField != "" {
					assert.Equal(t, tt.errField, err.(*ConfigError).Field)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
