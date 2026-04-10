// Package adaptive 提供基于系统负载的自适应限流和降级中间件.
package adaptive

import (
	"context"
	"errors"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Strategy 自适应限流策略类型.
type Strategy string

const (
	// StrategyCPU 基于 CPU 使用率的限流策略.
	StrategyCPU Strategy = "cpu"
	// StrategyLatency 基于延迟的限流策略.
	StrategyLatency Strategy = "latency"
	// StrategyErrorRate 基于错误率的限流策略.
	StrategyErrorRate Strategy = "error_rate"
	// StrategyComposite 组合多种策略的限流.
	StrategyComposite Strategy = "composite"
)

var (
	// ErrNilConfig 配置为空错误.
	ErrNilConfig = errors.New("adaptive: 配置不能为空")
	// ErrInvalidThreshold 阈值无效错误.
	ErrInvalidThreshold = errors.New("adaptive: 阈值无效")
)

// Config 自适应限流配置.
type Config struct {
	// Strategy 限流策略.
	Strategy Strategy

	// CPUThreshold CPU 使用率阈值，0.8 表示 80%.
	CPUThreshold float64

	// LatencyThreshold P99 延迟阈值.
	LatencyThreshold time.Duration

	// ErrorRateThreshold 错误率阈值，0.1 表示 10%.
	ErrorRateThreshold float64

	// WindowSize 指标采集窗口大小.
	WindowSize time.Duration

	// CooldownPeriod 触发限流后的冷却时间.
	CooldownPeriod time.Duration

	// DegradeHandler 降级时的降级处理器.
	DegradeHandler http.Handler
}

// Option 配置选项函数.
type Option func(*Limiter)

// WithLogger 设置日志记录器.
func WithLogger(l *log.Logger) Option {
	return func(lim *Limiter) {
		if l != nil {
			lim.logger = l
		}
	}
}

// MetricsCollector 指标采集器接口.
type MetricsCollector interface {
	// OnAllow 请求被允许通过时调用.
	OnAllow()
	// OnDrop 请求被拒绝时调用.
	OnDrop()
}

// WithMetricsCollector 设置指标采集器.
func WithMetricsCollector(mc MetricsCollector) Option {
	return func(lim *Limiter) {
		if mc != nil {
			lim.metricsCollector = mc
		}
	}
}

// Status 限流器当前状态.
type Status struct {
	// IsLimiting 是否正在限流.
	IsLimiting bool
	// CurrentCPU 当前 CPU 使用率.
	CurrentCPU float64
	// CurrentLatencyP99 当前 P99 延迟.
	CurrentLatencyP99 time.Duration
	// CurrentErrorRate 当前错误率.
	CurrentErrorRate float64
	// TotalRequests 总请求数.
	TotalRequests int64
	// DroppedRequests 被丢弃的请求数.
	DroppedRequests int64
}

// Limiter 自适应限流器.
type Limiter struct {
	cfg *Config

	latencyTracker   *latencyTracker
	errorRateTracker *errorRateTracker

	// CPU 负载估算
	cpuMu      sync.Mutex
	lastCPUIdle uint64
	lastCPUTotal uint64
	cpuUsage   float64

	// 冷却控制
	limiting     atomic.Bool
	cooldownEnd  atomic.Int64 // unix nano

	logger           *log.Logger
	metricsCollector MetricsCollector

	nowFunc func() time.Time // 方便测试

	// 后台 CPU 采样控制
	stopCPUSampler chan struct{}
	cpuSamplerDone chan struct{}
}

// New 创建自适应限流器.
func New(cfg *Config, opts ...Option) (*Limiter, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	windowSize := cfg.WindowSize
	if windowSize <= 0 {
		windowSize = 10 * time.Second
	}
	cooldown := cfg.CooldownPeriod
	if cooldown <= 0 {
		cooldown = 5 * time.Second
	}
	cfgCopy := *cfg
	cfgCopy.WindowSize = windowSize
	cfgCopy.CooldownPeriod = cooldown

	l := &Limiter{
		cfg:              &cfgCopy,
		latencyTracker:   newLatencyTracker(windowSize),
		errorRateTracker: newErrorRateTracker(windowSize),
		nowFunc:          time.Now,
		stopCPUSampler:   make(chan struct{}),
		cpuSamplerDone:   make(chan struct{}),
	}

	for _, opt := range opts {
		opt(l)
	}

	// 初始化 CPU 采样并启动后台定时采样
	l.sampleCPU()
	go l.cpuSamplerLoop()

	return l, nil
}

// validateConfig 验证配置有效性.
func validateConfig(cfg *Config) error {
	switch cfg.Strategy {
	case StrategyCPU:
		if cfg.CPUThreshold <= 0 || cfg.CPUThreshold > 1 {
			return ErrInvalidThreshold
		}
	case StrategyLatency:
		if cfg.LatencyThreshold <= 0 {
			return ErrInvalidThreshold
		}
	case StrategyErrorRate:
		if cfg.ErrorRateThreshold <= 0 || cfg.ErrorRateThreshold > 1 {
			return ErrInvalidThreshold
		}
	case StrategyComposite:
		// 至少需要一个阈值
		hasThreshold := false
		if cfg.CPUThreshold > 0 && cfg.CPUThreshold <= 1 {
			hasThreshold = true
		}
		if cfg.LatencyThreshold > 0 {
			hasThreshold = true
		}
		if cfg.ErrorRateThreshold > 0 && cfg.ErrorRateThreshold <= 1 {
			hasThreshold = true
		}
		if !hasThreshold {
			return ErrInvalidThreshold
		}
	default:
		return ErrInvalidThreshold
	}
	return nil
}

// cpuSamplerLoop 后台定时采样 CPU 使用率，每秒采样一次.
func (l *Limiter) cpuSamplerLoop() {
	defer close(l.cpuSamplerDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCPUSampler:
			return
		case <-ticker.C:
			l.sampleCPU()
		}
	}
}

// Close 停止后台 CPU 采样 goroutine，释放资源.
func (l *Limiter) Close() {
	close(l.stopCPUSampler)
	<-l.cpuSamplerDone
}

// Allow 检查当前系统负载是否允许请求通过.
func (l *Limiter) Allow() bool {
	now := l.nowFunc()

	// 检查是否在冷却期内
	if l.limiting.Load() {
		cdEnd := l.cooldownEnd.Load()
		if now.UnixNano() < cdEnd {
			l.errorRateTracker.recordDrop()
			if l.metricsCollector != nil {
				l.metricsCollector.OnDrop()
			}
			return false
		}
		l.limiting.Store(false)
	}

	shouldLimit := l.shouldLimit()
	if shouldLimit {
		l.limiting.Store(true)
		l.cooldownEnd.Store(now.Add(l.cfg.CooldownPeriod).UnixNano())
		l.errorRateTracker.recordDrop()
		if l.metricsCollector != nil {
			l.metricsCollector.OnDrop()
		}
		return false
	}

	if l.metricsCollector != nil {
		l.metricsCollector.OnAllow()
	}
	return true
}

// shouldLimit 根据策略判断是否需要限流.
func (l *Limiter) shouldLimit() bool {
	switch l.cfg.Strategy {
	case StrategyCPU:
		return l.cpuLoad() >= l.cfg.CPUThreshold
	case StrategyLatency:
		return l.latencyTracker.percentile(0.99) >= l.cfg.LatencyThreshold
	case StrategyErrorRate:
		return l.errorRateTracker.errorRate() >= l.cfg.ErrorRateThreshold
	case StrategyComposite:
		if l.cfg.CPUThreshold > 0 && l.cpuLoad() >= l.cfg.CPUThreshold {
			return true
		}
		if l.cfg.LatencyThreshold > 0 && l.latencyTracker.percentile(0.99) >= l.cfg.LatencyThreshold {
			return true
		}
		if l.cfg.ErrorRateThreshold > 0 && l.errorRateTracker.errorRate() >= l.cfg.ErrorRateThreshold {
			return true
		}
		return false
	default:
		return false
	}
}

// cpuLoad 获取当前 CPU 负载估算值.
// 在非 Linux 平台使用 goroutine 数量与 NumCPU 的比值作为估算.
func (l *Limiter) cpuLoad() float64 {
	l.cpuMu.Lock()
	defer l.cpuMu.Unlock()
	return l.cpuUsage
}

// sampleCPU 采样 CPU 使用率.
// 使用 goroutine 数量与可用 CPU 数的比值作为跨平台估算.
// 注意：此方法为近似值，适用于自适应限流的粗粒度判断。
// 在 Linux 平台可考虑读取 /proc/stat 获取更精确的 CPU 使用率。
// 当前实现通过后台每秒定时采样（cpuSamplerLoop），避免在请求热路径中实时计算。
func (l *Limiter) sampleCPU() {
	numCPU := runtime.NumCPU()
	numGoroutine := runtime.NumGoroutine()
	// 用 goroutine 数 / (NumCPU * 基准系数) 估算负载
	// 基准系数取 256, 即 256 个 goroutine/核 视为满载
	const factor = 256.0
	usage := float64(numGoroutine) / (float64(numCPU) * factor)
	if usage > 1.0 {
		usage = 1.0
	}

	l.cpuMu.Lock()
	l.cpuUsage = usage
	l.cpuMu.Unlock()
}

// Middleware 返回 HTTP 中间件.
func (l *Limiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow() {
				if l.cfg.DegradeHandler != nil {
					l.cfg.DegradeHandler.ServeHTTP(w, r)
					return
				}
				http.Error(w, "服务过载，请稍后重试", http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GRPCUnaryInterceptor 返回 gRPC 一元拦截器.
func (l *Limiter) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !l.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "服务过载，请稍后重试")
		}
		return handler(ctx, req)
	}
}

// RecordLatency 记录延迟样本.
func (l *Limiter) RecordLatency(d time.Duration) {
	l.latencyTracker.record(d)
}

// RecordError 记录错误事件.
func (l *Limiter) RecordError() {
	l.errorRateTracker.recordError()
}

// RecordSuccess 记录成功事件.
func (l *Limiter) RecordSuccess() {
	l.errorRateTracker.recordSuccess()
}

// Status 获取限流器当前状态.
func (l *Limiter) Status() *Status {
	return &Status{
		IsLimiting:        l.limiting.Load(),
		CurrentCPU:        l.cpuLoad(),
		CurrentLatencyP99: l.latencyTracker.percentile(0.99),
		CurrentErrorRate:  l.errorRateTracker.errorRate(),
		TotalRequests:     l.errorRateTracker.total.Load(),
		DroppedRequests:   l.errorRateTracker.dropped.Load(),
	}
}
