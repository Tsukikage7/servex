package slo

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewTracker_NilObjective(t *testing.T) {
	_, err := NewTracker([]*Objective{nil})
	if err != ErrNilObjective {
		t.Fatalf("expected ErrNilObjective, got %v", err)
	}
}

func TestNewTracker_InvalidTarget(t *testing.T) {
	tests := []struct {
		name   string
		target float64
	}{
		{"zero", 0},
		{"negative", -0.5},
		{"over 1", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTracker([]*Objective{{Name: "test", Target: tt.target, Window: 24 * time.Hour}})
			if err != ErrInvalidTarget {
				t.Fatalf("expected ErrInvalidTarget, got %v", err)
			}
		})
	}
}

func TestNewTracker_ValidTarget1(t *testing.T) {
	// Target = 1.0 should be valid (100% SLO)
	tracker, err := NewTracker([]*Objective{{Name: "perfect", Target: 1.0, Window: 24 * time.Hour}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
}

func TestRecord_ObjectiveNotFound(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "availability", Target: 0.999, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tracker.Record(t.Context(), "nonexistent", true)
	if err != ErrObjectiveNotFound {
		t.Fatalf("expected ErrObjectiveNotFound, got %v", err)
	}
}

func TestRecord_AndStatus(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "availability", Target: 0.999, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()

	// 记录 999 个好事件和 1 个坏事件
	for i := 0; i < 999; i++ {
		if err := tracker.Record(ctx, "availability", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := tracker.Record(ctx, "availability", false); err != nil {
		t.Fatal(err)
	}

	st, err := tracker.Status("availability")
	if err != nil {
		t.Fatal(err)
	}

	if st.TotalEvents != 1000 {
		t.Fatalf("expected 1000 total events, got %d", st.TotalEvents)
	}
	if st.GoodEvents != 999 {
		t.Fatalf("expected 999 good events, got %d", st.GoodEvents)
	}
	if st.BadEvents != 1 {
		t.Fatalf("expected 1 bad event, got %d", st.BadEvents)
	}

	// SLI = 999/1000 = 0.999, 刚好等于目标，不应违反
	if st.SLIValue != 0.999 {
		t.Fatalf("expected SLI 0.999, got %f", st.SLIValue)
	}
	if st.IsBreaching {
		t.Fatal("expected not breaching at target boundary")
	}
}

func TestErrorBudget(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()

	// 记录 95 好 + 5 坏 = 5% 错误率
	for i := 0; i < 95; i++ {
		tracker.Record(ctx, "api", true)
	}
	for i := 0; i < 5; i++ {
		tracker.Record(ctx, "api", false)
	}

	st, err := tracker.Status("api")
	if err != nil {
		t.Fatal(err)
	}

	// 错误预算 = 1 - 0.99 = 0.01
	if diff := st.ErrorBudget - 0.01; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("expected error budget ~0.01, got %f", st.ErrorBudget)
	}

	// 实际错误率 = 5/100 = 0.05
	// 消耗比 = 0.05 / 0.01 = 5.0
	// 剩余预算 = 1 - 5.0 = 0已耗尽
	if st.ErrorBudgetRemaining != 0 {
		t.Fatalf("expected error budget remaining 0, got %f", st.ErrorBudgetRemaining)
	}

	// BurnRate = 5.0
	if diff := st.BurnRate - 5.0; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("expected burn rate ~5.0, got %f", st.BurnRate)
	}

	// 应违反 SLO
	if !st.IsBreaching {
		t.Fatal("expected breaching")
	}
}

func TestBurnRate(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()

	// 正好在预算内：1% 错误率
	for i := 0; i < 99; i++ {
		tracker.Record(ctx, "api", true)
	}
	tracker.Record(ctx, "api", false)

	st, err := tracker.Status("api")
	if err != nil {
		t.Fatal(err)
	}

	// 实际错误率 = 0.01，预算 = 0.01
	// BurnRate = 1.0
	if diff := st.BurnRate - 1.0; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("expected burn rate ~1.0, got %f", st.BurnRate)
	}

	// 剩余预算 = 1 - 1.0 = 0
	if st.ErrorBudgetRemaining > 1e-9 {
		t.Fatalf("expected error budget remaining ~0, got %f", st.ErrorBudgetRemaining)
	}
}

func TestIsBreaching(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 无事件，不应违反
	if tracker.IsBreaching("api") {
		t.Fatal("expected not breaching with no events")
	}

	// 不存在的目标
	if tracker.IsBreaching("nonexistent") {
		t.Fatal("expected not breaching for nonexistent objective")
	}

	ctx := t.Context()
	// 注入大量错误
	for i := 0; i < 10; i++ {
		tracker.Record(ctx, "api", false)
	}

	if !tracker.IsBreaching("api") {
		t.Fatal("expected breaching after many errors")
	}
}

func TestOnBreach(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	var breached bool
	var mu sync.Mutex
	tracker.OnBreach(func(st *Status) {
		mu.Lock()
		breached = true
		mu.Unlock()
	})

	ctx := t.Context()

	// 先记录一些好事件，使得有基础数据
	for i := 0; i < 10; i++ {
		tracker.Record(ctx, "api", true)
	}

	// 记录大量坏事件以超过阈值
	for i := 0; i < 20; i++ {
		tracker.Record(ctx, "api", false)
	}

	mu.Lock()
	result := breached
	mu.Unlock()

	if !result {
		t.Fatal("expected breach callback to be called")
	}
}

func TestAllStatuses(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
		{Name: "web", Target: 0.999, Window: 7 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	statuses := tracker.AllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
}

func TestStatus_NotFound(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = tracker.Status("nonexistent")
	if err != ErrObjectiveNotFound {
		t.Fatalf("expected ErrObjectiveNotFound, got %v", err)
	}
}

func TestPrometheusCollector(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	}, WithPrometheusNamespace("test"))
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	tracker.Record(ctx, "api", true)
	tracker.Record(ctx, "api", false)

	collector := tracker.PrometheusCollector()
	if collector == nil {
		t.Fatal("expected non-nil collector")
	}

	// 验证可以注册到 Prometheus
	reg := prometheus.NewRegistry()
	if err := reg.Register(collector); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}

	// 验证可以采集指标
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected some metrics")
	}
}

func TestWithOptions(t *testing.T) {
	var logged bool
	logger := func(format string, v ...any) {
		logged = true
	}

	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	},
		WithLogger(logger),
		WithCheckInterval(30*time.Second),
		WithPrometheusNamespace("custom"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tracker.namespace != "custom" {
		t.Fatalf("expected namespace 'custom', got %q", tracker.namespace)
	}
	if tracker.checkInterval != 30*time.Second {
		t.Fatalf("expected check interval 30s, got %v", tracker.checkInterval)
	}

	// 验证 logger 被设置
	if tracker.printf == nil {
		t.Fatal("expected printf to be set")
	}
	tracker.printf("test %s", "log")
	if !logged {
		t.Fatal("expected logger to be called")
	}
}

func TestConcurrentRecording(t *testing.T) {
	tracker, err := NewTracker([]*Objective{
		{Name: "api", Target: 0.99, Window: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				tracker.Record(ctx, "api", j%10 != 0)
			}
		})
	}

	wg.Wait()

	st, err := tracker.Status("api")
	if err != nil {
		t.Fatal(err)
	}

	if st.TotalEvents != 10000 {
		t.Fatalf("expected 10000 total events, got %d", st.TotalEvents)
	}
}

func TestEmptyTracker(t *testing.T) {
	tracker, err := NewTracker(nil)
	if err != nil {
		t.Fatal(err)
	}

	statuses := tracker.AllStatuses()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses, got %d", len(statuses))
	}
}
