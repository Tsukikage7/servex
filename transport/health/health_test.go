package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus(t *testing.T) {
	assert.Equal(t, Status("UP"), StatusUp)
	assert.Equal(t, Status("DOWN"), StatusDown)
	assert.Equal(t, Status("UNKNOWN"), StatusUnknown)
}

func TestNew(t *testing.T) {
	h := New()
	assert.NotNil(t, h)
	assert.Equal(t, 5*time.Second, h.timeout)
}

func TestNew_WithTimeout(t *testing.T) {
	h := New(WithTimeout(10 * time.Second))
	assert.Equal(t, 10*time.Second, h.timeout)
}

func TestHealth_Liveness_NoCheckers(t *testing.T) {
	h := New()
	resp := h.Liveness(t.Context())

	assert.Equal(t, StatusUp, resp.Status)
	assert.Empty(t, resp.Checks)
}

func TestHealth_Readiness_NoCheckers(t *testing.T) {
	h := New()
	resp := h.Readiness(t.Context())

	assert.Equal(t, StatusUp, resp.Status)
	assert.Empty(t, resp.Checks)
}

func TestHealth_Liveness_AllUp(t *testing.T) {
	checker1 := NewAlwaysUpChecker("checker1")
	checker2 := NewAlwaysUpChecker("checker2")

	h := New(WithLivenessChecker(checker1, checker2))
	resp := h.Liveness(t.Context())

	assert.Equal(t, StatusUp, resp.Status)
	assert.Len(t, resp.Checks, 2)
	assert.Equal(t, StatusUp, resp.Checks["checker1"].Status)
	assert.Equal(t, StatusUp, resp.Checks["checker2"].Status)
}

func TestHealth_Readiness_OneDown(t *testing.T) {
	upChecker := NewAlwaysUpChecker("up")
	downChecker := NewCheckerFunc("down", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDown, Message: "service unavailable"}
	})

	h := New(WithReadinessChecker(upChecker, downChecker))
	resp := h.Readiness(t.Context())

	assert.Equal(t, StatusDown, resp.Status)
	assert.Equal(t, StatusUp, resp.Checks["up"].Status)
	assert.Equal(t, StatusDown, resp.Checks["down"].Status)
	assert.Equal(t, "service unavailable", resp.Checks["down"].Message)
}

func TestHealth_AddCheckers(t *testing.T) {
	h := New()

	h.AddLivenessChecker(NewAlwaysUpChecker("live"))
	h.AddReadinessChecker(NewAlwaysUpChecker("ready"))

	liveResp := h.Liveness(t.Context())
	readyResp := h.Readiness(t.Context())

	assert.Len(t, liveResp.Checks, 1)
	assert.Len(t, readyResp.Checks, 1)
}

func TestHealth_IsHealthy(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		h := New(WithReadinessChecker(NewAlwaysUpChecker("test")))
		assert.True(t, h.IsHealthy(t.Context()))
	})

	t.Run("unhealthy", func(t *testing.T) {
		downChecker := NewCheckerFunc("down", func(ctx context.Context) CheckResult {
			return CheckResult{Status: StatusDown}
		})
		h := New(WithReadinessChecker(downChecker))
		assert.False(t, h.IsHealthy(t.Context()))
	})
}

func TestHealth_Timeout(t *testing.T) {
	slowChecker := NewCheckerFunc("slow", func(ctx context.Context) CheckResult {
		select {
		case <-ctx.Done():
			return CheckResult{Status: StatusDown, Message: "timeout"}
		case <-time.After(5 * time.Second):
			return CheckResult{Status: StatusUp}
		}
	})

	h := New(
		WithTimeout(100*time.Millisecond),
		WithReadinessChecker(slowChecker),
	)

	start := time.Now()
	resp := h.Readiness(t.Context())
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 500*time.Millisecond)
	assert.Equal(t, StatusDown, resp.Status)
}

func TestCheckerFunc(t *testing.T) {
	checker := NewCheckerFunc("test", func(ctx context.Context) CheckResult {
		return CheckResult{
			Status:  StatusUp,
			Message: "all good",
			Details: map[string]any{"version": "1.0"},
		}
	})

	assert.Equal(t, "test", checker.Name())

	result := checker.Check(t.Context())
	assert.Equal(t, StatusUp, result.Status)
	assert.Equal(t, "all good", result.Message)
	assert.Equal(t, "1.0", result.Details["version"])
}

func TestAlwaysUpChecker(t *testing.T) {
	checker := NewAlwaysUpChecker("always-up")

	assert.Equal(t, "always-up", checker.Name())
	result := checker.Check(t.Context())
	assert.Equal(t, StatusUp, result.Status)
}

func TestCheckResult_MarshalJSON(t *testing.T) {
	result := CheckResult{
		Status:   StatusUp,
		Message:  "test",
		Duration: 100 * time.Millisecond,
		Details:  map[string]any{"key": "value"},
	}

	data, err := result.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"UP"`)
	assert.Contains(t, string(data), `"duration":"100ms"`)
}

func TestResponse_MarshalJSON(t *testing.T) {
	resp := Response{
		Status:    StatusUp,
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:  50 * time.Millisecond,
		Checks: map[string]CheckResult{
			"test": {Status: StatusUp},
		},
	}

	data, err := resp.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"UP"`)
	assert.Contains(t, string(data), `"timestamp":"2024-01-01T12:00:00Z"`)
	assert.Contains(t, string(data), `"duration":"50ms"`)
}

// mockPinger 模拟 Pinger 接口.
type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func TestPingChecker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pinger := &mockPinger{}
		checker := NewPingChecker("test", pinger)

		assert.Equal(t, "test", checker.Name())
		result := checker.Check(t.Context())
		assert.Equal(t, StatusUp, result.Status)
	})

	t.Run("failure", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("connection refused")}
		checker := NewPingChecker("test", pinger)

		result := checker.Check(t.Context())
		assert.Equal(t, StatusDown, result.Status)
		assert.Equal(t, "检查失败", result.Message)
	})
}

func TestDBChecker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pinger := &mockPinger{}
		checker := NewDBChecker("postgres", pinger)

		assert.Equal(t, "postgres", checker.Name())
		result := checker.Check(t.Context())
		assert.Equal(t, StatusUp, result.Status)
		assert.Equal(t, "database", result.Details["type"])
	})

	t.Run("failure", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("connection refused")}
		checker := NewDBChecker("postgres", pinger)

		result := checker.Check(t.Context())
		assert.Equal(t, StatusDown, result.Status)
		assert.Equal(t, "database", result.Details["type"])
	})
}

func TestRedisChecker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pinger := &mockPinger{}
		checker := NewRedisChecker("redis", pinger)

		assert.Equal(t, "redis", checker.Name())
		result := checker.Check(t.Context())
		assert.Equal(t, StatusUp, result.Status)
		assert.Equal(t, "redis", result.Details["type"])
	})

	t.Run("failure", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("connection refused")}
		checker := NewRedisChecker("redis", pinger)

		result := checker.Check(t.Context())
		assert.Equal(t, StatusDown, result.Status)
	})
}

func TestCompositeChecker(t *testing.T) {
	t.Run("all up", func(t *testing.T) {
		composite := NewCompositeChecker("composite",
			NewAlwaysUpChecker("c1"),
			NewAlwaysUpChecker("c2"),
		)

		assert.Equal(t, "composite", composite.Name())
		result := composite.Check(t.Context())
		assert.Equal(t, StatusUp, result.Status)
	})

	t.Run("one down", func(t *testing.T) {
		downChecker := NewCheckerFunc("down", func(ctx context.Context) CheckResult {
			return CheckResult{Status: StatusDown}
		})

		composite := NewCompositeChecker("composite",
			NewAlwaysUpChecker("up"),
			downChecker,
		)

		result := composite.Check(t.Context())
		assert.Equal(t, StatusDown, result.Status)
	})

	t.Run("unknown status", func(t *testing.T) {
		unknownChecker := NewCheckerFunc("unknown", func(ctx context.Context) CheckResult {
			return CheckResult{Status: StatusUnknown}
		})

		composite := NewCompositeChecker("composite",
			NewAlwaysUpChecker("up"),
			unknownChecker,
		)

		result := composite.Check(t.Context())
		assert.Equal(t, StatusUnknown, result.Status)
	})
}

func TestHealth_Concurrent(t *testing.T) {
	h := New(
		WithReadinessChecker(NewAlwaysUpChecker("c1")),
		WithReadinessChecker(NewAlwaysUpChecker("c2")),
		WithReadinessChecker(NewAlwaysUpChecker("c3")),
	)

	// 并发执行多次检查
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp := h.Readiness(t.Context())
			assert.Equal(t, StatusUp, resp.Status)
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
