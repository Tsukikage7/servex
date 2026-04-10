package retry

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// Strategy 重试策略接口.
// Strategy 是不可变的，每次 Report 返回新实例.
type Strategy interface {
	Next() (time.Duration, bool)
	Report(err error) Strategy
}

type fixedStrategy struct {
	interval   time.Duration
	maxRetries int
	attempt    int
}

// NewFixedStrategy 创建固定间隔重试策略.
func NewFixedStrategy(interval time.Duration, maxRetries int) Strategy {
	return &fixedStrategy{
		interval:   interval,
		maxRetries: maxRetries,
	}
}

func (s *fixedStrategy) Next() (time.Duration, bool) {
	if s.attempt >= s.maxRetries {
		return 0, false
	}
	return s.interval, true
}

func (s *fixedStrategy) Report(_ error) Strategy {
	return &fixedStrategy{
		interval:   s.interval,
		maxRetries: s.maxRetries,
		attempt:    s.attempt + 1,
	}
}

type exponentialStrategy struct {
	initial    time.Duration
	maxDelay   time.Duration
	maxRetries int
	attempt    int
}

// NewExponentialStrategy 创建指数退避重试策略.
func NewExponentialStrategy(initial, maxDelay time.Duration, maxRetries int) Strategy {
	return &exponentialStrategy{
		initial:    initial,
		maxDelay:   maxDelay,
		maxRetries: maxRetries,
	}
}

func (s *exponentialStrategy) Next() (time.Duration, bool) {
	if s.attempt >= s.maxRetries {
		return 0, false
	}
	delay := time.Duration(float64(s.initial) * math.Pow(2, float64(s.attempt)))
	if delay > s.maxDelay {
		delay = s.maxDelay
	}
	// Full jitter: 随机化延迟到 [0, delay] 范围内，避免多客户端同步重试风暴
	delay = time.Duration(rand.Int64N(int64(delay) + 1))
	return delay, true
}

func (s *exponentialStrategy) Report(_ error) Strategy {
	return &exponentialStrategy{
		initial:    s.initial,
		maxDelay:   s.maxDelay,
		maxRetries: s.maxRetries,
		attempt:    s.attempt + 1,
	}
}

type adaptiveStrategy struct {
	base       Strategy
	windowSize int
	threshold  int
	errors     []bool // 构造后不可变
}

// NewAdaptiveStrategy 在滑动窗口内统计错误次数，超过阈值时停止重试.
func NewAdaptiveStrategy(base Strategy, windowSize, threshold int) Strategy {
	return &adaptiveStrategy{
		base:       base,
		windowSize: windowSize,
		threshold:  threshold,
		errors:     make([]bool, 0, windowSize),
	}
}

func (s *adaptiveStrategy) Next() (time.Duration, bool) {
	if s.countErrors() >= s.threshold {
		return 0, false
	}
	return s.base.Next()
}

func (s *adaptiveStrategy) Report(err error) Strategy {
	newErrors := make([]bool, len(s.errors), s.windowSize)
	copy(newErrors, s.errors)
	newErrors = append(newErrors, err != nil)
	if len(newErrors) > s.windowSize {
		newErrors = newErrors[len(newErrors)-s.windowSize:]
	}

	return &adaptiveStrategy{
		base:       s.base.Report(err),
		windowSize: s.windowSize,
		threshold:  s.threshold,
		errors:     newErrors,
	}
}

func (s *adaptiveStrategy) countErrors() int {
	count := 0
	for _, isErr := range s.errors {
		if isErr {
			count++
		}
	}
	return count
}

// RetryWithStrategy 使用指定策略执行带重试的函数.
func RetryWithStrategy(ctx context.Context, strategy Strategy, fn func() error) error {
	s := strategy
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		s = s.Report(err)
		delay, ok := s.Next()
		if !ok {
			return err
		}

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}
