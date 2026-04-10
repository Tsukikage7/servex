// Package circuitbreaker 实现熔断器模式.
// 熔断器有三个状态：
//   - Closed（关闭）：正常工作，计数失败次数
//   - Open（开路）：拒绝所有请求，等待超时后转 HalfOpen
//   - HalfOpen（半开）：放行少量请求测试服务是否恢复
package circuitbreaker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// State 熔断器状态.
type State string

const (
	// StateClosed 关闭状态：正常处理请求.
	StateClosed State = "closed"
	// StateOpen 开路状态：拒绝所有请求.
	StateOpen State = "open"
	// StateHalfOpen 半开状态：放行探测请求.
	StateHalfOpen State = "half_open"
)

// CircuitBreaker 熔断器接口.
type CircuitBreaker interface {
	// Execute 在熔断器保护下执行 fn.
	Execute(ctx context.Context, fn func() error) error
	// State 返回当前熔断器状态.
	State() State
	// Reset 手动将熔断器重置为 Closed 状态.
	Reset()
}

// Options 熔断器配置.
type Options struct {
	// FailureThreshold 连续失败 N 次后开路，默认 5.
	FailureThreshold int
	// SuccessThreshold HalfOpen 时连续成功 M 次后关路，默认 2.
	SuccessThreshold int
	// OpenTimeout Open 状态超时后转 HalfOpen，默认 10s.
	OpenTimeout time.Duration
	// IsFailure 自定义失败判断函数，默认所有非 nil 错误均为失败.
	IsFailure func(error) bool
}

// Option 配置函数.
type Option func(*Options)

// WithFailureThreshold 设置失败阈值.
func WithFailureThreshold(n int) Option {
	return func(o *Options) { o.FailureThreshold = n }
}

// WithSuccessThreshold 设置成功阈值.
func WithSuccessThreshold(n int) Option {
	return func(o *Options) { o.SuccessThreshold = n }
}

// WithOpenTimeout 设置开路超时时间.
func WithOpenTimeout(d time.Duration) Option {
	return func(o *Options) { o.OpenTimeout = d }
}

// WithIsFailure 设置自定义失败判断函数.
func WithIsFailure(fn func(error) bool) Option {
	return func(o *Options) { o.IsFailure = fn }
}

// Breaker 熔断器实现.
type Breaker struct {
	opts Options

	mu              sync.Mutex
	state           State
	failureCount    int
	successCount    int
	lastStateChange time.Time

	// halfOpenInFlight 限制 HalfOpen 状态下并发探测请求数量
	halfOpenInFlight atomic.Int32
}

// 编译期接口合规检查.
var _ CircuitBreaker = (*Breaker)(nil)

// New 创建熔断器.
func New(opts ...Option) *Breaker {
	o := Options{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		OpenTimeout:      10 * time.Second,
		IsFailure:        func(err error) bool { return err != nil },
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Breaker{
		opts:            o,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Execute 在熔断器保护下执行 fn.
func (b *Breaker) Execute(ctx context.Context, fn func() error) error {
	if err := b.beforeExecute(); err != nil {
		return err
	}

	err := fn()
	b.afterExecute(err)
	return err
}

// State 返回当前状态.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tryTransitionToHalfOpen()
	return b.state
}

// Reset 手动重置为 Closed 状态.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.toState(StateClosed)
}

// beforeExecute 执行前检查状态.
func (b *Breaker) beforeExecute() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tryTransitionToHalfOpen()

	switch b.state {
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		// HalfOpen 状态只允许一次探测请求，使用原子计数器限制并发
		if b.halfOpenInFlight.Add(1) > 1 {
			b.halfOpenInFlight.Add(-1)
			return ErrCircuitOpen
		}
		return nil
	default:
		return nil
	}
}

// afterExecute 根据执行结果更新状态.
func (b *Breaker) afterExecute(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	isFailure := b.opts.IsFailure(err)

	switch b.state {
	case StateClosed:
		if isFailure {
			b.failureCount++
			if b.failureCount >= b.opts.FailureThreshold {
				b.toState(StateOpen)
			}
		} else {
			b.failureCount = 0
		}

	case StateHalfOpen:
		if isFailure {
			// 探测失败，重新开路
			b.halfOpenInFlight.Store(0)
			b.toState(StateOpen)
		} else {
			b.successCount++
			if b.successCount >= b.opts.SuccessThreshold {
				// 探测成功足够次数，关路
				b.halfOpenInFlight.Store(0)
				b.toState(StateClosed)
			} else {
				// 允许下一个探测请求
				b.halfOpenInFlight.Add(-1)
			}
		}
	}
}

// tryTransitionToHalfOpen 检查 Open 状态是否超时，超时则转 HalfOpen.
// 调用前必须持有 mu 锁.
func (b *Breaker) tryTransitionToHalfOpen() {
	if b.state == StateOpen && time.Since(b.lastStateChange) >= b.opts.OpenTimeout {
		b.toState(StateHalfOpen)
	}
}

// toState 切换状态，重置计数器.
// 调用前必须持有 mu 锁.
func (b *Breaker) toState(s State) {
	b.state = s
	b.lastStateChange = time.Now()
	b.failureCount = 0
	b.successCount = 0
}
