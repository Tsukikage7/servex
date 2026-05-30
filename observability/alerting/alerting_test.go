package alerting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockProvider 模拟指标提供者.
type mockProvider struct {
	mu     sync.Mutex
	values map[string]float64
	errs   map[string]error
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		values: make(map[string]float64),
		errs:   make(map[string]error),
	}
}

func (m *mockProvider) Query(_ context.Context, metric string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.errs[metric]; ok {
		return 0, err
	}
	v, ok := m.values[metric]
	if !ok {
		return 0, errors.New("metric not found")
	}
	return v, nil
}

func (m *mockProvider) Set(metric string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.errs, metric)
	m.values[metric] = value
}

func (m *mockProvider) SetError(metric string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errs[metric] = err
}

// mockNotifier 模拟通知器.
type mockNotifier struct {
	mu     sync.Mutex
	alerts []*Alert
}

func (n *mockNotifier) Notify(_ context.Context, alert *Alert) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.alerts = append(n.alerts, alert)
	return nil
}

func (n *mockNotifier) getAlerts() []*Alert {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]*Alert, len(n.alerts))
	copy(result, n.alerts)
	return result
}

func TestNew(t *testing.T) {
	p := newMockProvider()
	e := New(p)
	if e == nil {
		t.Fatal("New() 返回 nil")
	}
	if e.provider != p {
		t.Error("provider 未正确设置")
	}
}

func TestNewWithOptions(t *testing.T) {
	p := newMockProvider()
	var logged bool
	notifier := &mockNotifier{}

	e := New(p,
		WithLogger(func(format string, v ...any) { logged = true; _ = format; _ = v }),
		WithNotifier(notifier),
		WithDefaultEvalInterval(30*time.Second),
		WithHistorySize(500),
	)

	if e.notifier == nil {
		t.Error("notifier 未设置")
	}
	if e.defaultEvalInterval != 30*time.Second {
		t.Errorf("defaultEvalInterval 期望 30s, 实际 %v", e.defaultEvalInterval)
	}
	if e.historySize != 500 {
		t.Errorf("historySize 期望 500, 实际 %d", e.historySize)
	}
	// 触发日志验证.
	e.printf("test %s", "msg")
	if !logged {
		t.Error("logger 未正确调用")
	}
}

func TestAddRule(t *testing.T) {
	e := New(newMockProvider())

	rule := &Rule{
		ID:   "rule-1",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
	}

	if err := e.AddRule(rule); err != nil {
		t.Fatalf("AddRule() 失败: %v", err)
	}

	// 重复添加.
	if err := e.AddRule(rule); !errors.Is(err, ErrDuplicateRule) {
		t.Errorf("重复添加期望 ErrDuplicateRule, 实际 %v", err)
	}
}

func TestAddRuleInvalidCondition(t *testing.T) {
	e := New(newMockProvider())

	// nil 规则.
	if err := e.AddRule(nil); !errors.Is(err, ErrInvalidCondition) {
		t.Errorf("nil 规则期望 ErrInvalidCondition, 实际 %v", err)
	}

	// 空 ID.
	if err := e.AddRule(&Rule{Condition: Condition{Metric: "m", Operator: OpGT}}); !errors.Is(err, ErrInvalidCondition) {
		t.Errorf("空 ID 期望 ErrInvalidCondition, 实际 %v", err)
	}

	// 空 Metric.
	if err := e.AddRule(&Rule{ID: "r1"}); !errors.Is(err, ErrInvalidCondition) {
		t.Errorf("空 Metric 期望 ErrInvalidCondition, 实际 %v", err)
	}

	// 无效运算符.
	err := e.AddRule(&Rule{ID: "r1", Condition: Condition{Metric: "m", Operator: "INVALID"}})
	if !errors.Is(err, ErrInvalidCondition) {
		t.Errorf("无效运算符期望 ErrInvalidCondition, 实际 %v", err)
	}
}

func TestRemoveRule(t *testing.T) {
	e := New(newMockProvider())

	rule := &Rule{
		ID:   "rule-1",
		Name: "test",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
	}

	_ = e.AddRule(rule)

	if err := e.RemoveRule("rule-1"); err != nil {
		t.Fatalf("RemoveRule() 失败: %v", err)
	}

	// 删除不存在的规则.
	if err := e.RemoveRule("nonexistent"); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("删除不存在规则期望 ErrRuleNotFound, 实际 %v", err)
	}
}

func TestGetRule(t *testing.T) {
	e := New(newMockProvider())

	rule := &Rule{
		ID:   "rule-1",
		Name: "test",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
	}

	_ = e.AddRule(rule)

	got, err := e.GetRule("rule-1")
	if err != nil {
		t.Fatalf("GetRule() 失败: %v", err)
	}
	if got.ID != "rule-1" {
		t.Errorf("GetRule() ID 期望 rule-1, 实际 %s", got.ID)
	}

	_, err = e.GetRule("nonexistent")
	if !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("获取不存在规则期望 ErrRuleNotFound, 实际 %v", err)
	}
}

func TestListRules(t *testing.T) {
	e := New(newMockProvider())

	for i := 0; i < 3; i++ {
		_ = e.AddRule(&Rule{
			ID:   fmt.Sprintf("rule-%d", i),
			Name: fmt.Sprintf("test-%d", i),
			Type: RuleThreshold,
			Condition: Condition{
				Metric:    "cpu_usage",
				Operator:  OpGT,
				Threshold: 80,
			},
		})
	}

	rules := e.ListRules()
	if len(rules) != 3 {
		t.Errorf("ListRules() 期望 3 条, 实际 %d 条", len(rules))
	}
}

func TestThresholdAlert(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 50)

	e := New(provider)
	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0, // 立即触发
	})

	ctx := t.Context()

	// 值低于阈值，不触发.
	alerts, err := e.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate() 失败: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("阈值未超过时不应有告警, 实际 %d 个", len(alerts))
	}

	// 值超过阈值，第一次评估进入 Pending.
	provider.Set("cpu_usage", 90)
	alerts, err = e.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate() 失败: %v", err)
	}
	if len(alerts) == 0 {
		t.Fatal("阈值超过时应产生告警")
	}
	if alerts[0].State != StatePending {
		t.Errorf("首次超阈值期望 Pending, 实际 %s", alerts[0].State)
	}

	// 第二次评估，For=0 应进入 Firing.
	alerts, err = e.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate() 失败: %v", err)
	}
	found := false
	for _, a := range alerts {
		if a.RuleID == "high-cpu" && a.State == StateFiring {
			found = true
			break
		}
	}
	if !found {
		t.Error("期望 high-cpu 规则进入 Firing 状态")
	}
}

func TestThresholdAlertWithPendingDuration(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 90)

	e := New(provider)
	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 100 * time.Millisecond,
	})

	ctx := t.Context()

	// 第一次评估：进入 Pending.
	alerts, _ := e.Evaluate(ctx)
	if len(alerts) == 0 {
		t.Fatal("第一次评估应产生 Pending 告警")
	}
	if alerts[0].State != StatePending {
		t.Errorf("期望 Pending 状态, 实际 %s", alerts[0].State)
	}

	// 第二次评估未超过 For 时间：仍然 Pending.
	alerts, _ = e.Evaluate(ctx)
	if len(alerts) == 0 || alerts[0].State != StatePending {
		t.Error("For 时间内应保持 Pending")
	}

	// 等待超过 For 时间后评估：进入 Firing.
	time.Sleep(150 * time.Millisecond)
	alerts, _ = e.Evaluate(ctx)
	if len(alerts) == 0 {
		t.Fatal("超过 For 时间后应产生告警")
	}
	found := false
	for _, a := range alerts {
		if a.RuleID == "high-cpu" && a.State == StateFiring {
			found = true
		}
	}
	if !found {
		t.Error("超过 For 时间后应进入 Firing 状态")
	}
}

func TestRateAlert(t *testing.T) {
	provider := newMockProvider()
	provider.Set("error_rate", 0.01)

	e := New(provider)
	_ = e.AddRule(&Rule{
		ID:   "high-error-rate",
		Name: "High Error Rate",
		Type: RuleRate,
		Condition: Condition{
			Metric:    "error_rate",
			Operator:  OpGT,
			Threshold: 0.05,
		},
		For: 0,
	})

	ctx := t.Context()

	// 低于阈值.
	alerts, _ := e.Evaluate(ctx)
	if len(alerts) != 0 {
		t.Error("速率低于阈值不应告警")
	}

	// 超过阈值，第一次 Pending.
	provider.Set("error_rate", 0.1)
	alerts, _ = e.Evaluate(ctx)
	if len(alerts) == 0 {
		t.Fatal("速率超过阈值应告警")
	}
	if alerts[0].State != StatePending {
		t.Errorf("首次期望 Pending, 实际 %s", alerts[0].State)
	}

	// 第二次 Firing.
	alerts, _ = e.Evaluate(ctx)
	if len(alerts) == 0 {
		t.Fatal("第二次评估应产生告警")
	}
	if alerts[0].State != StateFiring {
		t.Errorf("期望 Firing, 实际 %s", alerts[0].State)
	}
}

func TestAbsenceAlert(t *testing.T) {
	provider := newMockProvider()
	// 不设置任何值，Query 会返回 error.

	e := New(provider)
	_ = e.AddRule(&Rule{
		ID:   "metric-absent",
		Name: "Metric Absent",
		Type: RuleAbsence,
		Condition: Condition{
			Metric:    "heartbeat",
			Operator:  OpGT, // 缺失检测中运算符不影响判定
			Threshold: 0,
		},
		For: 0,
	})

	ctx := t.Context()

	// 指标缺失 → 第一次 Pending.
	alerts, _ := e.Evaluate(ctx)
	if len(alerts) == 0 {
		t.Fatal("指标缺失时应产生告警")
	}
	if alerts[0].State != StatePending {
		t.Errorf("首次评估期望 Pending, 实际 %s", alerts[0].State)
	}
	// 第二次 PendingFor=0，handleAbsence 需检查 For=0.
	alerts, _ = e.Evaluate(ctx)
	if len(alerts) == 0 {
		t.Fatal("第二次评估指标仍缺失应产生告警")
	}
	// For=0 时 handleAbsence 中 now.Sub(pendingAt) >= 0 成立，应 Firing.
	if alerts[0].State != StateFiring {
		t.Errorf("For=0 第二次评估期望 Firing, 实际 %s", alerts[0].State)
	}

	// 指标恢复 → 查询成功 → handleResolution → Resolved.
	provider.Set("heartbeat", 1)
	alerts, _ = e.Evaluate(ctx)
	foundResolved := false
	for _, a := range alerts {
		if a.RuleID == "metric-absent" && a.State == StateResolved {
			foundResolved = true
		}
	}
	if !foundResolved {
		// 打印实际状态辅助调试.
		for _, a := range alerts {
			t.Logf("alert: ruleID=%s state=%s", a.RuleID, a.State)
		}
		t.Error("指标恢复后应进入 Resolved 状态")
	}
}

func TestAlertResolution(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 90)

	e := New(provider)
	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0,
	})

	ctx := t.Context()

	// 触发告警：第一次 Pending，第二次 Firing (For=0).
	e.Evaluate(ctx)              // → Pending
	alerts, _ := e.Evaluate(ctx) // → Firing
	hasFiring := false
	for _, a := range alerts {
		if a.State == StateFiring {
			hasFiring = true
		}
	}
	if !hasFiring {
		t.Error("应有 Firing 状态告警")
	}

	// 值恢复.
	provider.Set("cpu_usage", 50)
	alerts, _ = e.Evaluate(ctx)
	foundResolved := false
	for _, a := range alerts {
		if a.RuleID == "high-cpu" && a.State == StateResolved {
			foundResolved = true
		}
	}
	if !foundResolved {
		t.Error("值恢复后应进入 Resolved 状态")
	}

	// Resolved 后再次评估应回到无告警.
	provider.Set("cpu_usage", 50)
	alerts, _ = e.Evaluate(ctx)
	for _, a := range alerts {
		if a.RuleID == "high-cpu" && (a.State == StateFiring || a.State == StatePending) {
			t.Error("Resolved 之后不应再有活跃告警")
		}
	}
}

func TestNotifierIntegration(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 90)

	notifier := &mockNotifier{}
	e := New(provider, WithNotifier(notifier))

	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0,
	})

	ctx := t.Context()
	e.Evaluate(ctx) // → Pending
	e.Evaluate(ctx) // → Firing, triggers notify

	// 等待异步通知.
	time.Sleep(50 * time.Millisecond)

	notified := notifier.getAlerts()
	if len(notified) == 0 {
		t.Error("Firing 时应触发通知")
	}
}

func TestEvaluateMultipleRules(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 90)
	provider.Set("memory_usage", 95)
	provider.Set("disk_usage", 30)

	e := New(provider)

	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0,
	})

	_ = e.AddRule(&Rule{
		ID:   "high-memory",
		Name: "High Memory",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "memory_usage",
			Operator:  OpGT,
			Threshold: 90,
		},
		For: 0,
	})

	_ = e.AddRule(&Rule{
		ID:   "high-disk",
		Name: "High Disk",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "disk_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0,
	})

	ctx := t.Context()
	e.Evaluate(ctx)              // first eval → Pending for cpu and memory
	alerts, _ := e.Evaluate(ctx) // second eval → Firing for cpu and memory

	// cpu 和 memory 应触发，disk 不应触发.
	alertMap := make(map[string]AlertState)
	for _, a := range alerts {
		alertMap[a.RuleID] = a.State
	}

	if _, ok := alertMap["high-cpu"]; !ok {
		t.Error("high-cpu 应触发")
	}
	if _, ok := alertMap["high-memory"]; !ok {
		t.Error("high-memory 应触发")
	}
	if state, ok := alertMap["high-disk"]; ok && (state == StatePending || state == StateFiring) {
		t.Error("high-disk 不应触发")
	}
}

func TestStartStop(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 50)

	e := New(provider, WithDefaultEvalInterval(50*time.Millisecond))

	_ = e.AddRule(&Rule{
		ID:   "test",
		Name: "test",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
	})

	ctx := t.Context()

	// 启动.
	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start() 失败: %v", err)
	}

	// 重复启动.
	if err := e.Start(ctx); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("重复启动期望 ErrAlreadyRunning, 实际 %v", err)
	}

	// 等待几次评估.
	time.Sleep(200 * time.Millisecond)

	// 停止.
	if err := e.Stop(ctx); err != nil {
		t.Fatalf("Stop() 失败: %v", err)
	}

	// 重复停止.
	if err := e.Stop(ctx); !errors.Is(err, ErrNotRunning) {
		t.Errorf("重复停止期望 ErrNotRunning, 实际 %v", err)
	}
}

func TestStartNilProvider(t *testing.T) {
	e := New(nil)
	if err := e.Start(t.Context()); !errors.Is(err, ErrNilProvider) {
		t.Errorf("nil provider 启动期望 ErrNilProvider, 实际 %v", err)
	}
}

func TestEvaluateNilProvider(t *testing.T) {
	e := New(nil)
	_, err := e.Evaluate(t.Context())
	if !errors.Is(err, ErrNilProvider) {
		t.Errorf("nil provider 评估期望 ErrNilProvider, 实际 %v", err)
	}
}

func TestAlertHistory(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 90)

	e := New(provider, WithHistorySize(10))

	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0,
	})

	ctx := t.Context()

	// 触发告警：两次评估 Pending → Firing.
	e.Evaluate(ctx)
	e.Evaluate(ctx)

	history := e.AlertHistory(10)
	if len(history) == 0 {
		t.Error("Firing 后应有历史记录")
	}
	if history[0].State != StateFiring {
		t.Errorf("历史记录应为 Firing, 实际 %s", history[0].State)
	}

	// 恢复告警.
	provider.Set("cpu_usage", 50)
	e.Evaluate(ctx)

	history = e.AlertHistory(10)
	if len(history) < 2 {
		t.Error("Resolved 后应有两条历史记录")
	}

	// 限制返回条数.
	history = e.AlertHistory(1)
	if len(history) != 1 {
		t.Errorf("AlertHistory(1) 期望 1 条, 实际 %d 条", len(history))
	}
}

func TestActiveAlerts(t *testing.T) {
	provider := newMockProvider()
	provider.Set("cpu_usage", 90)

	e := New(provider)

	_ = e.AddRule(&Rule{
		ID:   "high-cpu",
		Name: "High CPU",
		Type: RuleThreshold,
		Condition: Condition{
			Metric:    "cpu_usage",
			Operator:  OpGT,
			Threshold: 80,
		},
		For: 0,
	})

	ctx := t.Context()

	// 触发告警：两次评估 Pending → Firing.
	e.Evaluate(ctx)
	e.Evaluate(ctx)

	active := e.ActiveAlerts()
	if len(active) == 0 {
		t.Error("Firing 状态应出现在 ActiveAlerts")
	}

	// 恢复.
	provider.Set("cpu_usage", 50)
	e.Evaluate(ctx)

	active = e.ActiveAlerts()
	if len(active) != 0 {
		t.Error("Resolved 后不应有活跃告警")
	}
}

func TestCompareValue(t *testing.T) {
	tests := []struct {
		value     float64
		op        Operator
		threshold float64
		want      bool
	}{
		{10, OpGT, 5, true},
		{5, OpGT, 10, false},
		{5, OpGTE, 5, true},
		{4, OpGTE, 5, false},
		{3, OpLT, 5, true},
		{5, OpLT, 5, false},
		{5, OpLTE, 5, true},
		{6, OpLTE, 5, false},
		{5, OpEQ, 5, true},
		{5, OpEQ, 6, false},
		{5, OpNEQ, 6, true},
		{5, OpNEQ, 5, false},
		{5, Operator("invalid"), 5, false},
	}

	for _, tt := range tests {
		got := compareValue(tt.value, tt.op, tt.threshold)
		if got != tt.want {
			t.Errorf("compareValue(%v, %q, %v) = %v, 期望 %v", tt.value, tt.op, tt.threshold, got, tt.want)
		}
	}
}

func TestIsValidOperator(t *testing.T) {
	valid := []Operator{OpGT, OpGTE, OpLT, OpLTE, OpEQ, OpNEQ}
	for _, op := range valid {
		if !isValidOperator(op) {
			t.Errorf("运算符 %q 应为有效", op)
		}
	}

	if isValidOperator("INVALID") {
		t.Error("INVALID 运算符不应有效")
	}
}

func TestCopyMap(t *testing.T) {
	original := map[string]string{"a": "1", "b": "2"}
	copied := copyMap(original)

	if len(copied) != len(original) {
		t.Error("拷贝后长度不一致")
	}

	// 修改拷贝不影响原始.
	copied["c"] = "3"
	if _, ok := original["c"]; ok {
		t.Error("修改拷贝不应影响原始 map")
	}

	// nil 输入.
	if copyMap(nil) != nil {
		t.Error("nil 输入应返回 nil")
	}
}

func TestWithLoggerNil(t *testing.T) {
	e := New(newMockProvider(), WithLogger(nil))
	if e.printf != nil {
		t.Error("nil logger 不应设置 printf")
	}
}

func TestWithNotifierNil(t *testing.T) {
	e := New(newMockProvider(), WithNotifier(nil))
	if e.notifier != nil {
		t.Error("nil notifier 不应设置")
	}
}

func TestWithDefaultEvalIntervalZero(t *testing.T) {
	e := New(newMockProvider(), WithDefaultEvalInterval(0))
	if e.defaultEvalInterval != 15*time.Second {
		t.Error("零值不应改变默认评估间隔")
	}
}

func TestWithHistorySizeZero(t *testing.T) {
	e := New(newMockProvider(), WithHistorySize(0))
	if e.historySize != 1000 {
		t.Error("零值不应改变默认历史大小")
	}
}
