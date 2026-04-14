// Package health 提供统一的健康检查功能，支持 HTTP 和 gRPC 协议.
package health

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Status 健康状态.
type Status string

const (
	// StatusUp 服务健康.
	StatusUp Status = "UP"
	// StatusDown 服务不健康.
	StatusDown Status = "DOWN"
	// StatusUnknown 状态未知.
	StatusUnknown Status = "UNKNOWN"
)

// CheckResult 单个检查器的检查结果.
type CheckResult struct {
	Status   Status         `json:"status"`
	Message  string         `json:"message,omitempty"`
	Duration time.Duration  `json:"-"`
	Details  map[string]any `json:"details,omitzero"`
}

// MarshalJSON 自定义 JSON 序列化，将 Duration 转为可读字符串.
func (r CheckResult) MarshalJSON() ([]byte, error) {
	type Alias CheckResult
	return json.Marshal(&struct {
		Alias
		Duration string `json:"duration,omitempty"`
	}{
		Alias:    Alias(r),
		Duration: r.Duration.String(),
	})
}

// Response 健康检查响应.
type Response struct {
	Status    Status                 `json:"status"`
	Version   string                 `json:"version,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"-"`
	Checks    map[string]CheckResult `json:"checks,omitzero"`
}

// MarshalJSON 自定义 JSON 序列化.
func (r Response) MarshalJSON() ([]byte, error) {
	type Alias Response
	return json.Marshal(&struct {
		Alias
		Timestamp string `json:"timestamp"`
		Duration  string `json:"duration"`
	}{
		Alias:     Alias(r),
		Timestamp: r.Timestamp.Format(time.RFC3339),
		Duration:  r.Duration.String(),
	})
}

// Checker 健康检查器接口.
type Checker interface {
	// Name 返回检查器名称.
	Name() string
	// Check 执行健康检查.
	Check(ctx context.Context) CheckResult
}

// CheckerFunc 函数类型检查器适配器.
type CheckerFunc struct {
	name string
	fn   func(ctx context.Context) CheckResult
}

// NewCheckerFunc 创建函数类型检查器.
func NewCheckerFunc(name string, fn func(ctx context.Context) CheckResult) *CheckerFunc {
	return &CheckerFunc{name: name, fn: fn}
}

// Name 返回检查器名称.
func (c *CheckerFunc) Name() string {
	return c.name
}

// Check 执行健康检查.
func (c *CheckerFunc) Check(ctx context.Context) CheckResult {
	return c.fn(ctx)
}

// Option Health 配置选项.
type Option func(*Health)

// Health 健康检查管理器.
type Health struct {
	mu                sync.RWMutex
	livenessCheckers  []Checker
	readinessCheckers []Checker
	timeout           time.Duration
	version           string
}

// New 创建健康检查管理器.
func New(opts ...Option) *Health {
	h := &Health{
		timeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithTimeout 设置检查超时时间.
func WithTimeout(d time.Duration) Option {
	return func(h *Health) {
		h.timeout = d
	}
}

// WithVersion 设置服务版本，健康检查响应中会携带此信息.
func WithVersion(v string) Option {
	return func(h *Health) {
		h.version = v
	}
}

// WithLivenessChecker 添加存活检查器.
func WithLivenessChecker(checkers ...Checker) Option {
	return func(h *Health) {
		h.livenessCheckers = append(h.livenessCheckers, checkers...)
	}
}

// WithReadinessChecker 添加就绪检查器.
func WithReadinessChecker(checkers ...Checker) Option {
	return func(h *Health) {
		h.readinessCheckers = append(h.readinessCheckers, checkers...)
	}
}

// AddLivenessChecker 动态添加存活检查器.
func (h *Health) AddLivenessChecker(checkers ...Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.livenessCheckers = append(h.livenessCheckers, checkers...)
}

// AddReadinessChecker 动态添加就绪检查器.
func (h *Health) AddReadinessChecker(checkers ...Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readinessCheckers = append(h.readinessCheckers, checkers...)
}

// Liveness 执行存活检查.
//
// 存活检查用于判断进程是否正常运行，通常用于 K8s livenessProbe.
// 如果没有注册任何存活检查器，默认返回 UP.
func (h *Health) Liveness(ctx context.Context) Response {
	h.mu.RLock()
	checkers := h.livenessCheckers
	h.mu.RUnlock()

	return h.runChecks(ctx, checkers)
}

// Readiness 执行就绪检查.
//
// 就绪检查用于判断服务是否可以接受流量，通常检查依赖（DB、Redis等）.
// 如果没有注册任何就绪检查器，默认返回 UP.
func (h *Health) Readiness(ctx context.Context) Response {
	h.mu.RLock()
	checkers := h.readinessCheckers
	h.mu.RUnlock()

	return h.runChecks(ctx, checkers)
}

// runChecks 并发执行所有检查器.
func (h *Health) runChecks(ctx context.Context, checkers []Checker) Response {
	start := time.Now()

	// 没有检查器时默认返回 UP
	if len(checkers) == 0 {
		return Response{
			Status:    StatusUp,
			Version:   h.version,
			Timestamp: start,
			Duration:  time.Since(start),
		}
	}

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	// 并发执行所有检查
	results := make(map[string]CheckResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, checker := range checkers {
		c := checker
		wg.Go(func() {
			checkStart := time.Now()
			result := c.Check(checkCtx)
			result.Duration = time.Since(checkStart)

			mu.Lock()
			results[c.Name()] = result
			mu.Unlock()
		})
	}

	wg.Wait()

	// 计算整体状态
	overallStatus := StatusUp
	for _, result := range results {
		if result.Status == StatusDown {
			overallStatus = StatusDown
			break
		}
		if result.Status == StatusUnknown {
			overallStatus = StatusUnknown
		}
	}

	return Response{
		Status:    overallStatus,
		Version:   h.version,
		Timestamp: start,
		Duration:  time.Since(start),
		Checks:    results,
	}
}

// IsHealthy 返回服务是否健康（就绪检查通过）.
func (h *Health) IsHealthy(ctx context.Context) bool {
	return h.Readiness(ctx).Status == StatusUp
}
