// Package slo 提供 SLO/SLI 追踪，支持错误预算计算和告警.
package slo

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// ErrObjectiveNotFound 目标未找到错误.
	ErrObjectiveNotFound = errors.New("slo: 目标未找到")
	// ErrInvalidTarget 目标值无效错误.
	ErrInvalidTarget = errors.New("slo: 目标值无效，应在 (0, 1] 范围内")
	// ErrNilObjective 目标为空错误.
	ErrNilObjective = errors.New("slo: 目标不能为空")
)

// Objective SLO 目标定义.
type Objective struct {
	// Name 目标名称.
	Name string
	// Target 目标值，如 0.999 表示 99.9%.
	Target float64
	// Window 统计窗口，如 30 天.
	Window time.Duration
	// Description 目标描述.
	Description string
}

// SLI 服务级别指标接口.
type SLI interface {
	// Record 记录一个好/坏事件.
	Record(ctx context.Context, good bool)
}

// Status SLO 当前状态.
type Status struct {
	// Objective 对应的 SLO 目标.
	Objective *Objective
	// TotalEvents 总事件数.
	TotalEvents int64
	// GoodEvents 好事件数.
	GoodEvents int64
	// BadEvents 坏事件数.
	BadEvents int64
	// SLIValue 当前 SLI 值.
	SLIValue float64
	// ErrorBudget 错误预算总量.
	ErrorBudget float64
	// ErrorBudgetRemaining 剩余错误预算.
	ErrorBudgetRemaining float64
	// BurnRate 错误预算消耗速率.
	BurnRate float64
	// IsBreaching 是否违反 SLO.
	IsBreaching bool
}

// Option 配置选项函数.
type Option func(*Tracker)

// WithLogger 设置日志记录器兼容标准库 log.Logger 签名.
func WithLogger(printf func(format string, v ...any)) Option {
	return func(t *Tracker) {
		if printf != nil {
			t.printf = printf
		}
	}
}

// WithCheckInterval 设置 SLO 检查间隔.
func WithCheckInterval(d time.Duration) Option {
	return func(t *Tracker) {
		if d > 0 {
			t.checkInterval = d
		}
	}
}

// WithPrometheusNamespace 设置 Prometheus 指标命名空间.
func WithPrometheusNamespace(ns string) Option {
	return func(t *Tracker) {
		if ns != "" {
			t.namespace = ns
		}
	}
}

// objectiveTracker 单个目标的追踪器.
type objectiveTracker struct {
	objective *Objective
	good      atomic.Int64
	bad       atomic.Int64
	onBreach  func(status *Status)
	mu        sync.RWMutex
	// lastReportedGood/Bad 记录上次 Collect 时已上报的事件数，用于增量更新 Counter.
	lastReportedGood int64
	lastReportedBad  int64
}

// Tracker SLO 追踪器.
type Tracker struct {
	mu        sync.RWMutex
	trackers  map[string]*objectiveTracker
	namespace string
	printf    func(format string, v ...any)

	checkInterval time.Duration

	// Prometheus 指标
	promOnce       sync.Once
	eventsTotal    *prometheus.CounterVec
	budgetRemGauge *prometheus.GaugeVec
	burnRateGauge  *prometheus.GaugeVec
}

// NewTracker 创建 SLO 追踪器.
func NewTracker(objectives []*Objective, opts ...Option) (*Tracker, error) {
	t := &Tracker{
		trackers:      make(map[string]*objectiveTracker),
		namespace:     "app",
		checkInterval: time.Minute,
	}

	for _, opt := range opts {
		opt(t)
	}

	for _, obj := range objectives {
		if obj == nil {
			return nil, ErrNilObjective
		}
		if obj.Target <= 0 || obj.Target > 1 {
			return nil, ErrInvalidTarget
		}
		t.trackers[obj.Name] = &objectiveTracker{
			objective: obj,
		}
	}

	return t, nil
}

// Record 记录一个事件.
func (t *Tracker) Record(ctx context.Context, objectiveName string, good bool) error {
	t.mu.RLock()
	ot, ok := t.trackers[objectiveName]
	t.mu.RUnlock()
	if !ok {
		return ErrObjectiveNotFound
	}

	if good {
		ot.good.Add(1)
	} else {
		ot.bad.Add(1)
	}

	// 检查是否违反 SLO 并触发回调
	ot.mu.RLock()
	fn := ot.onBreach
	ot.mu.RUnlock()

	if fn != nil && !good {
		st := t.computeStatus(ot)
		if st.IsBreaching {
			fn(st)
		}
	}

	return nil
}

// Status 获取指定目标的状态.
func (t *Tracker) Status(objectiveName string) (*Status, error) {
	t.mu.RLock()
	ot, ok := t.trackers[objectiveName]
	t.mu.RUnlock()
	if !ok {
		return nil, ErrObjectiveNotFound
	}
	return t.computeStatus(ot), nil
}

// AllStatuses 获取所有目标的状态.
func (t *Tracker) AllStatuses() []*Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	statuses := make([]*Status, 0, len(t.trackers))
	for _, ot := range t.trackers {
		statuses = append(statuses, t.computeStatus(ot))
	}
	return statuses
}

// IsBreaching 检查指定目标是否违反 SLO.
func (t *Tracker) IsBreaching(objectiveName string) bool {
	st, err := t.Status(objectiveName)
	if err != nil {
		return false
	}
	return st.IsBreaching
}

// OnBreach 注册 SLO 违反时的回调函数.
func (t *Tracker) OnBreach(fn func(status *Status)) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, ot := range t.trackers {
		ot.mu.Lock()
		ot.onBreach = fn
		ot.mu.Unlock()
	}
}

// computeStatus 计算目标的当前状态.
// 注意：当前使用累计值计算 SLI，尚未实现滑动窗口语义Window 字段暂未生效.
func (t *Tracker) computeStatus(ot *objectiveTracker) *Status {
	good := ot.good.Load()
	bad := ot.bad.Load()
	total := good + bad

	var sliValue float64
	if total > 0 {
		sliValue = float64(good) / float64(total)
	} else {
		sliValue = 1.0 // 无事件时视为 100%
	}

	errorBudget := 1.0 - ot.objective.Target
	var errorBudgetRemaining float64
	var burnRate float64

	if total > 0 {
		actualErrorRate := float64(bad) / float64(total)
		if errorBudget > 0 {
			consumed := actualErrorRate / errorBudget
			errorBudgetRemaining = 1.0 - consumed
			if errorBudgetRemaining < 0 {
				errorBudgetRemaining = 0
			}
			burnRate = consumed
		}
	} else {
		errorBudgetRemaining = 1.0
	}

	return &Status{
		Objective:            ot.objective,
		TotalEvents:          total,
		GoodEvents:           good,
		BadEvents:            bad,
		SLIValue:             sliValue,
		ErrorBudget:          errorBudget,
		ErrorBudgetRemaining: errorBudgetRemaining,
		BurnRate:             burnRate,
		IsBreaching:          sliValue < ot.objective.Target,
	}
}

// sloCollector 实现 prometheus.Collector 接口.
type sloCollector struct {
	tracker *Tracker
}

// PrometheusCollector 返回实现 prometheus.Collector 接口的采集器.
func (t *Tracker) PrometheusCollector() prometheus.Collector {
	t.promOnce.Do(func() {
		t.eventsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: t.namespace,
				Subsystem: "slo",
				Name:      "events_total",
				Help:      "Total SLO events",
			},
			[]string{"name", "result"},
		)
		t.budgetRemGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: t.namespace,
				Subsystem: "slo",
				Name:      "error_budget_remaining",
				Help:      "Remaining error budget ratio",
			},
			[]string{"name"},
		)
		t.burnRateGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: t.namespace,
				Subsystem: "slo",
				Name:      "burn_rate",
				Help:      "Error budget burn rate",
			},
			[]string{"name"},
		)
	})
	return &sloCollector{tracker: t}
}

// Describe 实现 prometheus.Collector 接口.
func (c *sloCollector) Describe(ch chan<- *prometheus.Desc) {
	c.tracker.eventsTotal.Describe(ch)
	c.tracker.budgetRemGauge.Describe(ch)
	c.tracker.burnRateGauge.Describe(ch)
}

// Collect 实现 prometheus.Collector 接口.
func (c *sloCollector) Collect(ch chan<- prometheus.Metric) {
	t := c.tracker

	// 先在 RLock 下复制 trackers 快照，避免持 RLock 时再获取 ot.mu.Lock 导致死锁
	t.mu.RLock()
	type entry struct {
		name string
		ot   *objectiveTracker
	}
	snapshot := make([]entry, 0, len(t.trackers))
	for name, ot := range t.trackers {
		snapshot = append(snapshot, entry{name: name, ot: ot})
	}
	t.mu.RUnlock()

	for _, e := range snapshot {
		name := e.name
		ot := e.ot
		good := ot.good.Load()
		bad := ot.bad.Load()

		// 计算自上次上报以来的增量并更新 Counter.
		ot.mu.Lock()
		goodDelta := good - ot.lastReportedGood
		badDelta := bad - ot.lastReportedBad
		ot.lastReportedGood = good
		ot.lastReportedBad = bad
		ot.mu.Unlock()

		if goodDelta > 0 {
			t.eventsTotal.WithLabelValues(name, "good").Add(float64(goodDelta))
		}
		if badDelta > 0 {
			t.eventsTotal.WithLabelValues(name, "bad").Add(float64(badDelta))
		}

		st := t.computeStatus(ot)
		t.budgetRemGauge.WithLabelValues(name).Set(st.ErrorBudgetRemaining)
		t.burnRateGauge.WithLabelValues(name).Set(st.BurnRate)
	}

	t.eventsTotal.Collect(ch)
	t.budgetRemGauge.Collect(ch)
	t.burnRateGauge.Collect(ch)
}
