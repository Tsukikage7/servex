package cache

import (
	"testing"
	"time"

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
	s.Equal(ErrNilConfig, err)
}

func (s *ConfigTestSuite) TestValidate_EmptyConfig() {
	config := &Config{}
	err := config.Validate()

	s.NoError(err)
}

func (s *ConfigTestSuite) TestValidate_InvalidType() {
	config := &Config{Type: "invalid"}
	err := config.Validate()

	s.Error(err)
	s.IsType(&ConfigError{}, err)
}

func (s *ConfigTestSuite) TestValidate_RedisWithoutAddr() {
	config := &Config{Type: TypeRedis, Addr: ""}
	err := config.Validate()

	s.Error(err)
	s.IsType(&ConfigError{}, err)
}

func (s *ConfigTestSuite) TestValidate_RedisWithAddr() {
	config := &Config{Type: TypeRedis, Addr: "localhost:6379"}
	err := config.Validate()

	s.NoError(err)
}

func (s *ConfigTestSuite) TestValidate_MemoryType() {
	config := &Config{Type: TypeMemory}
	err := config.Validate()

	s.NoError(err)
}

func (s *ConfigTestSuite) TestApplyDefaults() {
	config := &Config{}
	config.ApplyDefaults()

	s.Equal(TypeMemory, config.Type)
	s.Equal(DefaultPoolSize, config.PoolSize)
	s.Equal(DefaultTimeout, config.Timeout)
	s.Equal(DefaultReadTimeout, config.ReadTimeout)
	s.Equal(DefaultWriteTimeout, config.WriteTimeout)
	s.Equal(DefaultMaxRetries, config.MaxRetries)
	s.Equal(10000, config.MaxSize)
	s.Equal(time.Minute, config.CleanupInterval)
}

func (s *ConfigTestSuite) TestApplyDefaults_PreservesValues() {
	config := &Config{
		Type:     TypeMemory,
		PoolSize: 20,
		Timeout:  10 * time.Second,
		MaxSize:  5000,
	}
	config.ApplyDefaults()

	s.Equal(TypeMemory, config.Type)
	s.Equal(20, config.PoolSize)
	s.Equal(10*time.Second, config.Timeout)
	s.Equal(5000, config.MaxSize)
}

func (s *ConfigTestSuite) TestDefaultConfig() {
	config := DefaultConfig()

	s.NotNil(config)
	s.Equal(TypeMemory, config.Type)
	s.Equal(DefaultPoolSize, config.PoolSize)
}

func (s *ConfigTestSuite) TestNewRedisConfig() {
	config := NewRedisConfig("localhost:6379")

	s.NotNil(config)
	s.Equal(TypeRedis, config.Type)
	s.Equal("localhost:6379", config.Addr)
	s.Equal(DefaultPoolSize, config.PoolSize)
}

func (s *ConfigTestSuite) TestNewMemoryConfig() {
	config := NewMemoryConfig()

	s.NotNil(config)
	s.Equal(TypeMemory, config.Type)
	s.Equal(10000, config.MaxSize)
}

func (s *ConfigTestSuite) TestConfigError() {
	err := &ConfigError{Field: "addr", Message: "不能为空"}
	expected := "缓存配置错误 [addr]: 不能为空"

	s.Equal(expected, err.Error())
}
