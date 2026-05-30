package ratelimit

import (
	"fmt"
	"time"
)

const (
	// AlgorithmTokenBucket 令牌桶算法.
	AlgorithmTokenBucket = "token_bucket"
	// AlgorithmSlidingWindow 滑动窗口算法.
	AlgorithmSlidingWindow = "sliding_window"
	// AlgorithmFixedWindow 固定窗口算法.
	AlgorithmFixedWindow = "fixed_window"
	// AlgorithmDistributed 分布式限流算法.
	AlgorithmDistributed = "distributed"
)

// Config 限流配置.
type Config struct {
	// Algorithm 限流算法类型
	Algorithm string `mapstructure:"algorithm" json:"algorithm" yaml:"algorithm"`

	// Rate 速率令牌桶：每秒令牌数
	Rate float64 `mapstructure:"rate" json:"rate" yaml:"rate"`

	// Capacity 容量令牌桶：桶容量
	Capacity float64 `mapstructure:"capacity" json:"capacity" yaml:"capacity"`

	// Limit 限制窗口算法：窗口内最大请求数
	Limit int `mapstructure:"limit" json:"limit" yaml:"limit"`

	// Window 窗口大小
	Window time.Duration `mapstructure:"window" json:"window" yaml:"window"`

	// Prefix 分布式限流键前缀
	Prefix string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`

	// Counter 计数器实例分布式限流用
	Counter RateCounter `mapstructure:"-" json:"-" yaml:"-"`
}

// Validate 验证配置.
func (c *Config) Validate() error {
	switch c.Algorithm {
	case AlgorithmTokenBucket:
		if c.Rate <= 0 {
			return fmt.Errorf("%w: rate 必须大于 0", ErrInvalidConfig)
		}
		if c.Capacity <= 0 {
			return fmt.Errorf("%w: capacity 必须大于 0", ErrInvalidConfig)
		}
	case AlgorithmSlidingWindow, AlgorithmFixedWindow:
		if c.Limit <= 0 {
			return fmt.Errorf("%w: limit 必须大于 0", ErrInvalidConfig)
		}
		if c.Window <= 0 {
			return fmt.Errorf("%w: window 必须大于 0", ErrInvalidConfig)
		}
	case AlgorithmDistributed:
		if c.Counter == nil {
			return ErrNilCache
		}
		if c.Limit <= 0 {
			return fmt.Errorf("%w: limit 必须大于 0", ErrInvalidConfig)
		}
		if c.Window <= 0 {
			return fmt.Errorf("%w: window 必须大于 0", ErrInvalidConfig)
		}
	case "":
		return fmt.Errorf("%w: algorithm 不能为空", ErrInvalidConfig)
	default:
		return fmt.Errorf("%w: 不支持的算法类型 %s", ErrInvalidConfig, c.Algorithm)
	}
	return nil
}

// NewLimiter 根据配置创建限流器.
func NewLimiter(cfg *Config) (Limiter, error) {
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch cfg.Algorithm {
	case AlgorithmTokenBucket:
		return NewTokenBucket(cfg.Rate, cfg.Capacity), nil
	case AlgorithmSlidingWindow:
		return NewSlidingWindow(cfg.Limit, cfg.Window), nil
	case AlgorithmFixedWindow:
		return NewFixedWindow(cfg.Limit, cfg.Window), nil
	case AlgorithmDistributed:
		return NewDistributedLimiter(&DistributedConfig{
			Counter: cfg.Counter,
			Prefix:  cfg.Prefix,
			Limit:   cfg.Limit,
			Window:  cfg.Window,
		})
	default:
		return nil, fmt.Errorf("%w: 不支持的算法类型 %s", ErrInvalidConfig, cfg.Algorithm)
	}
}

// MustNewLimiter 根据配置创建限流器，失败时 panic.
func MustNewLimiter(cfg *Config) Limiter {
	limiter, err := NewLimiter(cfg)
	if err != nil {
		panic(err)
	}
	return limiter
}
