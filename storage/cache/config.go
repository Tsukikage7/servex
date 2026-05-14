package cache

import "time"

// Config 缓存配置.
type Config struct {
	Type         string        `json:"type" yaml:"type" toml:"type" mapstructure:"type"`
	Addr         string        `json:"addr" yaml:"addr" toml:"addr" mapstructure:"addr"`
	Password     string        `json:"-" yaml:"password" toml:"password" mapstructure:"password"`
	DB           int           `json:"db" yaml:"db" toml:"db" mapstructure:"db"`
	PoolSize     int           `json:"pool_size" yaml:"pool_size" toml:"pool_size" mapstructure:"pool_size"`
	Timeout      time.Duration `json:"timeout" yaml:"timeout" toml:"timeout" mapstructure:"timeout"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout" toml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" toml:"write_timeout" mapstructure:"write_timeout"`
	MaxRetries   int           `json:"max_retries" yaml:"max_retries" toml:"max_retries" mapstructure:"max_retries"`

	// 内存缓存专用
	MaxSize         int           `json:"max_size" yaml:"max_size" toml:"max_size" mapstructure:"max_size"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval" toml:"cleanup_interval" mapstructure:"cleanup_interval"`
}

// ConfigError 配置错误.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "缓存配置错误 [" + e.Field + "]: " + e.Message
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c == nil {
		return ErrNilConfig
	}

	if c.Type != "" && c.Type != TypeRedis && c.Type != TypeMemory {
		return &ConfigError{Field: "type", Message: "必须是 redis 或 memory"}
	}

	if c.Type == TypeRedis && c.Addr == "" {
		return &ConfigError{Field: "addr", Message: "Redis 地址不能为空"}
	}

	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	if c.Type == "" {
		c.Type = TypeMemory
	}

	if c.PoolSize <= 0 {
		c.PoolSize = DefaultPoolSize
	}

	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}

	if c.ReadTimeout <= 0 {
		c.ReadTimeout = DefaultReadTimeout
	}

	if c.WriteTimeout <= 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}

	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}

	// 内存缓存默认值
	if c.MaxSize <= 0 {
		c.MaxSize = 10000
	}

	if c.CleanupInterval <= 0 {
		c.CleanupInterval = time.Minute
	}
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	config := &Config{}
	config.ApplyDefaults()
	return config
}

// NewRedisConfig 创建 Redis 配置.
func NewRedisConfig(addr string) *Config {
	config := &Config{
		Type: TypeRedis,
		Addr: addr,
	}
	config.ApplyDefaults()
	return config
}

// NewMemoryConfig 创建内存缓存配置.
func NewMemoryConfig() *Config {
	config := &Config{
		Type: TypeMemory,
	}
	config.ApplyDefaults()
	return config
}
