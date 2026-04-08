package adaptive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNew_NilConfig(t *testing.T) {
	_, err := New(nil)
	if err != ErrNilConfig {
		t.Fatalf("expected ErrNilConfig, got %v", err)
	}
}

func TestNew_InvalidThreshold(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "cpu threshold zero",
			cfg:  &Config{Strategy: StrategyCPU, CPUThreshold: 0},
		},
		{
			name: "cpu threshold over 1",
			cfg:  &Config{Strategy: StrategyCPU, CPUThreshold: 1.5},
		},
		{
			name: "latency threshold zero",
			cfg:  &Config{Strategy: StrategyLatency, LatencyThreshold: 0},
		},
		{
			name: "error rate threshold zero",
			cfg:  &Config{Strategy: StrategyErrorRate, ErrorRateThreshold: 0},
		},
		{
			name: "error rate threshold over 1",
			cfg:  &Config{Strategy: StrategyErrorRate, ErrorRateThreshold: 1.5},
		},
		{
			name: "composite no threshold",
			cfg:  &Config{Strategy: StrategyComposite},
		},
		{
			name: "unknown strategy",
			cfg:  &Config{Strategy: "unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err != ErrInvalidThreshold {
				t.Fatalf("expected ErrInvalidThreshold, got %v", err)
			}
		})
	}
}

func TestNew_ValidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "cpu strategy",
			cfg:  &Config{Strategy: StrategyCPU, CPUThreshold: 0.8},
		},
		{
			name: "latency strategy",
			cfg:  &Config{Strategy: StrategyLatency, LatencyThreshold: 500 * time.Millisecond},
		},
		{
			name: "error rate strategy",
			cfg:  &Config{Strategy: StrategyErrorRate, ErrorRateThreshold: 0.1},
		},
		{
			name: "composite strategy",
			cfg: &Config{
				Strategy:           StrategyComposite,
				CPUThreshold:       0.8,
				LatencyThreshold:   500 * time.Millisecond,
				ErrorRateThreshold: 0.1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lim, err := New(tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lim == nil {
				t.Fatal("expected non-nil limiter")
			}
		})
	}
}

func TestAllow_ErrorRateStrategy(t *testing.T) {
	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Second,
		CooldownPeriod:     50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 初始状态应允许通过
	if !lim.Allow() {
		t.Fatal("expected allow on initial state")
	}

	// 注入大量错误以触发限流
	for i := 0; i < 100; i++ {
		lim.RecordError()
	}

	if lim.Allow() {
		t.Fatal("expected deny after high error rate")
	}

	st := lim.Status()
	if !st.IsLimiting {
		t.Fatal("expected IsLimiting=true")
	}
	if st.DroppedRequests == 0 {
		t.Fatal("expected dropped requests > 0")
	}
}

func TestAllow_LatencyStrategy(t *testing.T) {
	lim, err := New(&Config{
		Strategy:         StrategyLatency,
		LatencyThreshold: 100 * time.Millisecond,
		WindowSize:       time.Second,
		CooldownPeriod:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 初始无样本，应允许
	if !lim.Allow() {
		t.Fatal("expected allow on initial state")
	}

	// 注入高延迟样本
	for i := 0; i < 100; i++ {
		lim.RecordLatency(200 * time.Millisecond)
	}

	if lim.Allow() {
		t.Fatal("expected deny after high latency")
	}
}

func TestCooldownBehavior(t *testing.T) {
	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Second,
		CooldownPeriod:     100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 触发限流
	for i := 0; i < 100; i++ {
		lim.RecordError()
	}
	lim.Allow() // 触发限流

	// 在冷却期内应被拒绝
	if lim.Allow() {
		t.Fatal("expected deny during cooldown")
	}

	// 等待冷却期结束
	time.Sleep(150 * time.Millisecond)

	// 注入成功事件使错误率降低
	for i := 0; i < 200; i++ {
		lim.RecordSuccess()
	}

	// 冷却期后且错误率已降低，应允许
	if !lim.Allow() {
		t.Fatal("expected allow after cooldown with low error rate")
	}
}

func TestMiddleware_Allow(t *testing.T) {
	lim, err := New(&Config{
		Strategy:     StrategyCPU,
		CPUThreshold: 0.99, // 高阈值，几乎不会触发
		WindowSize:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := lim.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_DegradeHandler(t *testing.T) {
	degradeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("degraded"))
	})

	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.1,
		WindowSize:         time.Second,
		CooldownPeriod:     time.Second,
		DegradeHandler:     degradeHandler,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 触发限流
	for i := 0; i < 100; i++ {
		lim.RecordError()
	}

	handler := lim.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("normal"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from degrade handler, got %d", rec.Code)
	}
	if rec.Body.String() != "degraded" {
		t.Fatalf("expected 'degraded' body, got %q", rec.Body.String())
	}
}

func TestMiddleware_ServiceUnavailable(t *testing.T) {
	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.1,
		WindowSize:         time.Second,
		CooldownPeriod:     time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 触发限流
	for i := 0; i < 100; i++ {
		lim.RecordError()
	}

	handler := lim.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestGRPCUnaryInterceptor(t *testing.T) {
	lim, err := New(&Config{
		Strategy:     StrategyCPU,
		CPUThreshold: 0.99, // 高阈值，不会触发
		WindowSize:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	interceptor := lim.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestGRPCUnaryInterceptor_Denied(t *testing.T) {
	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.1,
		WindowSize:         time.Second,
		CooldownPeriod:     time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 触发限流
	for i := 0; i < 100; i++ {
		lim.RecordError()
	}

	interceptor := lim.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", st.Code())
	}
}

func TestStatus(t *testing.T) {
	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	lim.RecordSuccess()
	lim.RecordError()
	lim.RecordLatency(100 * time.Millisecond)

	st := lim.Status()
	if st.TotalRequests != 2 {
		t.Fatalf("expected TotalRequests=2, got %d", st.TotalRequests)
	}
	if st.CurrentErrorRate == 0 {
		t.Fatal("expected non-zero error rate")
	}
	if st.CurrentLatencyP99 == 0 {
		t.Fatal("expected non-zero latency p99")
	}
}

func TestCompositeStrategy(t *testing.T) {
	lim, err := New(&Config{
		Strategy:           StrategyComposite,
		CPUThreshold:       0.99, // 不会触发
		LatencyThreshold:   100 * time.Millisecond,
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Second,
		CooldownPeriod:     50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 初始应允许
	if !lim.Allow() {
		t.Fatal("expected allow initially")
	}

	// 注入高延迟触发限流
	for i := 0; i < 100; i++ {
		lim.RecordLatency(200 * time.Millisecond)
	}

	if lim.Allow() {
		t.Fatal("expected deny after high latency in composite")
	}
}

func TestWithMetricsCollector(t *testing.T) {
	var allows, drops int
	mc := &testMetricsCollector{
		onAllow: func() { allows++ },
		onDrop:  func() { drops++ },
	}

	lim, err := New(&Config{
		Strategy:           StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Second,
		CooldownPeriod:     time.Second,
	}, WithMetricsCollector(mc))
	if err != nil {
		t.Fatal(err)
	}

	lim.Allow() // 应成功
	if allows != 1 {
		t.Fatalf("expected 1 allow, got %d", allows)
	}

	// 触发限流
	for i := 0; i < 100; i++ {
		lim.RecordError()
	}
	lim.Allow() // 应被拒绝
	if drops < 1 {
		t.Fatalf("expected at least 1 drop, got %d", drops)
	}
}

type testMetricsCollector struct {
	onAllow func()
	onDrop  func()
}

func (m *testMetricsCollector) OnAllow() { m.onAllow() }
func (m *testMetricsCollector) OnDrop()  { m.onDrop() }
