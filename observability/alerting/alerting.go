// Package alerting 提供通用告警规则引擎，支持阈值、趋势和速率告警.
package alerting

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// AlertState 告警状态类型.
type AlertState string

const (
	// StateOK 正常状态.
	StateOK AlertState = "ok"
	// StatePending 待确认状态条件满足但未超过持续时间.
	StatePending AlertState = "pending"
	// StateFiring 告警触发状态.
	StateFiring AlertState = "firing"
	// StateResolved 告警已恢复状态.
	StateResolved AlertState = "resolved"
)

// RuleType 规则类型.
type RuleType string

const (
	// RuleThreshold 阈值告警规则.
	RuleThreshold RuleType = "threshold"
	// RuleRate 速率告警规则.
	RuleRate RuleType = "rate"
	// RuleAbsence 缺失检测告警规则.
	RuleAbsence RuleType = "absence"
)

// Operator 比较运算符类型.
type Operator string

const (
	// OpGT 大于.
	OpGT Operator = ">"
	// OpGTE 大于等于.
	OpGTE Operator = ">="
	// OpLT 小于.
	OpLT Operator = "<"
	// OpLTE 小于等于.
	OpLTE Operator = "<="
	// OpEQ 等于.
	OpEQ Operator = "=="
	// OpNEQ 不等于.
	OpNEQ Operator = "!="
)

var (
	// ErrNilProvider 指标提供者为空错误.
	ErrNilProvider = errors.New("alerting: 指标提供者不能为空")
	// ErrRuleNotFound 规则未找到错误.
	ErrRuleNotFound = errors.New("alerting: 规则未找到")
	// ErrDuplicateRule 规则已存在错误.
	ErrDuplicateRule = errors.New("alerting: 规则已存在")
	// ErrInvalidCondition 无效条件错误.
	ErrInvalidCondition = errors.New("alerting: 无效的告警条件")
	// ErrAlreadyRunning 引擎已运行错误.
	ErrAlreadyRunning = errors.New("alerting: 引擎已在运行")
	// ErrNotRunning 引擎未运行错误.
	ErrNotRunning = errors.New("alerting: 引擎未在运行")
)

// Condition 告警条件.
type Condition struct {
	// Metric 指标名称.
	Metric string
	// Operator 比较运算符.
	Operator Operator
	// Threshold 阈值.
	Threshold float64
	// Duration 条件必须持续满足的时间.
	Duration time.Duration
}

// Rule 告警规则.
type Rule struct {
	// ID 规则唯一标识.
	ID string
	// Name 规则名称.
	Name string
	// Type 规则类型.
	Type RuleType
	// Condition 告警条件.
	Condition Condition
	// Labels 标签.
	Labels map[string]string
	// Annotations 注解.
	Annotations map[string]string
	// EvalInterval 评估间隔.
	EvalInterval time.Duration
	// For 从 Pending 到 Firing 的等待时间.
	For time.Duration
}

// Alert 告警实例.
type Alert struct {
	// ID 告警唯一标识.
	ID string
	// RuleID 关联的规则 ID.
	RuleID string
	// State 当前状态.
	State AlertState
	// Value 当前指标值.
	Value float64
	// Labels 标签.
	Labels map[string]string
	// Annotations 注解.
	Annotations map[string]string
	// StartsAt 告警开始时间.
	StartsAt time.Time
	// EndsAt 告警结束时间.
	EndsAt time.Time
	// UpdatedAt 最后更新时间.
	UpdatedAt time.Time
}

// MetricProvider 指标数据提供者接口.
type MetricProvider interface {
	// Query 查询指标值.
	Query(ctx context.Context, metric string) (float64, error)
}

// Notifier 告警通知接口.
type Notifier interface {
	// Notify 发送告警通知.
	Notify(ctx context.Context, alert *Alert) error
}

// Option 配置选项函数.
type Option func(*Engine)

// WithLogger 设置日志记录器兼容标准库 log.Logger 签名.
func WithLogger(printf func(format string, v ...any)) Option {
	return func(e *Engine) {
		if printf != nil {
			e.printf = printf
		}
	}
}

// WithNotifier 设置告警通知器.
func WithNotifier(n Notifier) Option {
	return func(e *Engine) {
		if n != nil {
			e.notifier = n
		}
	}
}

// WithDefaultEvalInterval 设置默认评估间隔.
func WithDefaultEvalInterval(d time.Duration) Option {
	return func(e *Engine) {
		if d > 0 {
			e.defaultEvalInterval = d
		}
	}
}

// WithHistorySize 设置告警历史记录最大条数.
func WithHistorySize(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.historySize = n
		}
	}
}

// ruleState 规则运行时状态.
type ruleState struct {
	alert     *Alert
	pendingAt time.Time
}

// Engine 告警规则引擎.
type Engine struct {
	mu       sync.RWMutex
	provider MetricProvider
	notifier Notifier
	printf   func(format string, v ...any)

	rules    map[string]*Rule
	states   map[string]*ruleState
	history  []*Alert
	activeID int64

	defaultEvalInterval time.Duration
	historySize         int

	running bool
	cancel  context.CancelFunc
	done    chan struct{}
}

// New 创建告警规则引擎.
func New(provider MetricProvider, opts ...Option) *Engine {
	e := &Engine{
		provider:            provider,
		rules:               make(map[string]*Rule),
		states:              make(map[string]*ruleState),
		defaultEvalInterval: 15 * time.Second,
		historySize:         1000,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// AddRule 添加告警规则.
func (e *Engine) AddRule(rule *Rule) error {
	if rule == nil {
		return ErrInvalidCondition
	}
	if rule.ID == "" || rule.Condition.Metric == "" {
		return ErrInvalidCondition
	}
	if !isValidOperator(rule.Condition.Operator) {
		return fmt.Errorf("%w: 无效的运算符 %q", ErrInvalidCondition, rule.Condition.Operator)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[rule.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateRule, rule.ID)
	}

	e.rules[rule.ID] = rule
	e.states[rule.ID] = &ruleState{
		alert: &Alert{
			RuleID:      rule.ID,
			State:       StateOK,
			Labels:      copyMap(rule.Labels),
			Annotations: copyMap(rule.Annotations),
		},
	}

	return nil
}

// RemoveRule 移除告警规则.
func (e *Engine) RemoveRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[id]; !exists {
		return fmt.Errorf("%w: %s", ErrRuleNotFound, id)
	}

	delete(e.rules, id)
	delete(e.states, id)

	return nil
}

// GetRule 获取告警规则.
func (e *Engine) GetRule(id string) (*Rule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, exists := e.rules[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrRuleNotFound, id)
	}

	return rule, nil
}

// ListRules 列出所有告警规则.
func (e *Engine) ListRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*Rule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	return rules
}

// Start 启动评估循环.
func (e *Engine) Start(ctx context.Context) error {
	if e.provider == nil {
		return ErrNilProvider
	}

	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return ErrAlreadyRunning
	}
	e.running = true
	evalCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.done = make(chan struct{})
	e.mu.Unlock()

	go e.evalLoop(evalCtx)

	return nil
}

// Stop 停止评估循环.
func (e *Engine) Stop(_ context.Context) error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return ErrNotRunning
	}
	e.running = false
	e.cancel()
	done := e.done
	e.mu.Unlock()

	<-done
	return nil
}

// Evaluate 一次性评估所有规则.
func (e *Engine) Evaluate(ctx context.Context) ([]*Alert, error) {
	if e.provider == nil {
		return nil, ErrNilProvider
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.evaluateAll(ctx)
}

// ActiveAlerts 返回当前活跃的告警Pending 或 Firing.
func (e *Engine) ActiveAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var alerts []*Alert
	for _, rs := range e.states {
		if rs.alert.State == StatePending || rs.alert.State == StateFiring {
			a := copyAlert(rs.alert)
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// AlertHistory 返回告警历史记录.
func (e *Engine) AlertHistory(limit int) []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.history) {
		limit = len(e.history)
	}

	// 返回最近的 limit 条记录.
	start := len(e.history) - limit
	result := make([]*Alert, limit)
	for i, a := range e.history[start:] {
		result[i] = copyAlert(a)
	}
	return result
}

// evalLoop 持续评估循环.
func (e *Engine) evalLoop(ctx context.Context) {
	defer close(e.done)

	e.mu.RLock()
	interval := e.defaultEvalInterval
	e.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 先在读锁下复制规则快照，避免在持锁期间执行网络 IO
			e.mu.RLock()
			snapshot := make(map[string]*Rule, len(e.rules))
			stateSnap := make(map[string]*ruleState, len(e.states))
			for id, rule := range e.rules {
				snapshot[id] = rule
				stateSnap[id] = e.states[id]
			}
			e.mu.RUnlock()

			alerts, err := e.evaluateSnapshot(ctx, snapshot, stateSnap)
			if err != nil {
				if e.printf != nil {
					e.printf("alerting: 评估失败: %v", err)
				}
				continue
			}
			_ = alerts
		}
	}
}

// evaluateAll 评估所有规则调用者需持有写锁.
func (e *Engine) evaluateAll(ctx context.Context) ([]*Alert, error) {
	return e.evaluateSnapshot(ctx, e.rules, e.states)
}

// evaluateSnapshot 在无锁环境下评估规则快照，避免持锁执行网络 IO.
func (e *Engine) evaluateSnapshot(ctx context.Context, rules map[string]*Rule, states map[string]*ruleState) ([]*Alert, error) {
	now := time.Now()
	var alerts []*Alert

	for id, rule := range rules {
		rs := states[id]
		alert, err := e.evaluateRule(ctx, rule, rs, now)
		if err != nil {
			if e.printf != nil {
				e.printf("alerting: 评估规则 %s 失败: %v", id, err)
			}
			// 对于缺失检测规则，查询失败表示指标缺失.
			if rule.Type == RuleAbsence {
				alert = e.handleAbsence(rule, rs, now)
			} else {
				continue
			}
		}
		if alert != nil {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

// evaluateRule 评估单条规则.
func (e *Engine) evaluateRule(ctx context.Context, rule *Rule, rs *ruleState, now time.Time) (*Alert, error) {
	value, err := e.provider.Query(ctx, rule.Condition.Metric)
	if err != nil {
		return nil, err
	}

	// 缺失检测：能查到值则恢复.
	if rule.Type == RuleAbsence {
		return e.handleResolution(rule, rs, value, now), nil
	}

	conditionMet := compareValue(value, rule.Condition.Operator, rule.Condition.Threshold)

	if conditionMet {
		return e.handleConditionMet(rule, rs, value, now), nil
	}

	return e.handleResolution(rule, rs, value, now), nil
}

// handleAbsence 处理缺失检测查询失败视为指标缺失.
func (e *Engine) handleAbsence(rule *Rule, rs *ruleState, now time.Time) *Alert {
	switch rs.alert.State {
	case StateOK, StateResolved:
		rs.alert.State = StatePending
		rs.pendingAt = now
		rs.alert.StartsAt = now
		rs.alert.UpdatedAt = now
		return copyAlert(rs.alert)
	case StatePending:
		if rule.For > 0 && now.Sub(rs.pendingAt) >= rule.For {
			rs.alert.State = StateFiring
			rs.alert.UpdatedAt = now
			e.addHistory(rs.alert)
			e.notify(rs.alert)
			return copyAlert(rs.alert)
		}
		if rule.For == 0 {
			rs.alert.State = StateFiring
			rs.alert.UpdatedAt = now
			e.addHistory(rs.alert)
			e.notify(rs.alert)
			return copyAlert(rs.alert)
		}
		rs.alert.UpdatedAt = now
		return copyAlert(rs.alert)
	case StateFiring:
		rs.alert.UpdatedAt = now
		return copyAlert(rs.alert)
	}
	return nil
}

// handleConditionMet 处理条件满足的情况.
func (e *Engine) handleConditionMet(rule *Rule, rs *ruleState, value float64, now time.Time) *Alert {
	rs.alert.Value = value

	switch rs.alert.State {
	case StateOK, StateResolved:
		rs.alert.State = StatePending
		rs.pendingAt = now
		rs.alert.StartsAt = now
		rs.alert.UpdatedAt = now
		e.activeID++
		rs.alert.ID = fmt.Sprintf("%s-%d", rule.ID, e.activeID)
		return copyAlert(rs.alert)
	case StatePending:
		if rule.For > 0 && now.Sub(rs.pendingAt) >= rule.For {
			rs.alert.State = StateFiring
			rs.alert.UpdatedAt = now
			e.addHistory(rs.alert)
			e.notify(rs.alert)
			return copyAlert(rs.alert)
		}
		// For 为 0 时立即触发.
		if rule.For == 0 {
			rs.alert.State = StateFiring
			rs.alert.UpdatedAt = now
			e.addHistory(rs.alert)
			e.notify(rs.alert)
			return copyAlert(rs.alert)
		}
		rs.alert.UpdatedAt = now
		return copyAlert(rs.alert)
	case StateFiring:
		rs.alert.UpdatedAt = now
		return copyAlert(rs.alert)
	}

	return nil
}

// handleResolution 处理条件恢复.
func (e *Engine) handleResolution(rule *Rule, rs *ruleState, value float64, now time.Time) *Alert {
	rs.alert.Value = value

	switch rs.alert.State {
	case StatePending:
		// 条件不再满足，回退到 OK.
		rs.alert.State = StateOK
		rs.alert.UpdatedAt = now
		return nil
	case StateFiring:
		rs.alert.State = StateResolved
		rs.alert.EndsAt = now
		rs.alert.UpdatedAt = now
		e.addHistory(rs.alert)
		e.notify(rs.alert)
		return copyAlert(rs.alert)
	}

	return nil
}

// addHistory 添加告警到历史记录.
func (e *Engine) addHistory(alert *Alert) {
	a := copyAlert(alert)
	e.history = append(e.history, a)

	// 超出容量时移除最旧的记录.
	if len(e.history) > e.historySize {
		e.history = e.history[len(e.history)-e.historySize:]
	}
}

// notify 发送告警通知.
func (e *Engine) notify(alert *Alert) {
	if e.notifier == nil {
		return
	}
	a := copyAlert(alert)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if e.printf != nil {
					e.printf("alerting: 发送通知 panic: %v", r)
				}
			}
		}()
		if err := e.notifier.Notify(context.Background(), a); err != nil {
			if e.printf != nil {
				e.printf("alerting: 发送通知失败: %v", err)
			}
		}
	}()
}

// floatEpsilon 浮点数比较精度.
const floatEpsilon = 1e-9

// compareValue 比较值与阈值.
func compareValue(value float64, op Operator, threshold float64) bool {
	switch op {
	case OpGT:
		return value > threshold
	case OpGTE:
		return value >= threshold
	case OpLT:
		return value < threshold
	case OpLTE:
		return value <= threshold
	case OpEQ:
		return math.Abs(value-threshold) < floatEpsilon
	case OpNEQ:
		return math.Abs(value-threshold) >= floatEpsilon
	default:
		return false
	}
}

// isValidOperator 检查运算符是否有效.
func isValidOperator(op Operator) bool {
	switch op {
	case OpGT, OpGTE, OpLT, OpLTE, OpEQ, OpNEQ:
		return true
	default:
		return false
	}
}

// copyMap 拷贝 map.
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// copyAlert 拷贝告警实例.
func copyAlert(a *Alert) *Alert {
	if a == nil {
		return nil
	}
	return &Alert{
		ID:          a.ID,
		RuleID:      a.RuleID,
		State:       a.State,
		Value:       a.Value,
		Labels:      copyMap(a.Labels),
		Annotations: copyMap(a.Annotations),
		StartsAt:    a.StartsAt,
		EndsAt:      a.EndsAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
