package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type StrategyTestSuite struct {
	suite.Suite
}

func TestStrategySuite(t *testing.T) {
	suite.Run(t, new(StrategyTestSuite))
}

func (s *StrategyTestSuite) TestFixedStrategy_Next() {
	st := NewFixedStrategy(100*time.Millisecond, 3)

	delay, ok := st.Next()
	s.True(ok)
	s.Equal(100*time.Millisecond, delay)
}

func (s *StrategyTestSuite) TestFixedStrategy_ExhaustedAfterMax() {
	st := NewFixedStrategy(50*time.Millisecond, 2)

	st = st.Report(errors.New("err"))
	st = st.Report(errors.New("err"))

	_, ok := st.Next()
	s.False(ok)
}

func (s *StrategyTestSuite) TestFixedStrategy_Immutable() {
	st := NewFixedStrategy(100*time.Millisecond, 3)
	st2 := st.Report(errors.New("err"))

	delay1, ok1 := st.Next()
	s.True(ok1)
	s.Equal(100*time.Millisecond, delay1)

	delay2, ok2 := st2.Next()
	s.True(ok2)
	s.Equal(100*time.Millisecond, delay2)
}

func (s *StrategyTestSuite) TestFixedStrategy_ZeroRetries() {
	st := NewFixedStrategy(100*time.Millisecond, 0)
	_, ok := st.Next()
	s.False(ok)
}

func (s *StrategyTestSuite) TestExponentialStrategy_Backoff() {
	st := NewExponentialStrategy(100*time.Millisecond, 10*time.Second, 5)

	// attempt 0: [0, 100ms] (full jitter)
	delay, ok := st.Next()
	s.True(ok)
	s.LessOrEqual(delay, 100*time.Millisecond)
	s.GreaterOrEqual(delay, time.Duration(0))

	// attempt 1: [0, 200ms]
	st = st.Report(errors.New("err"))
	delay, ok = st.Next()
	s.True(ok)
	s.LessOrEqual(delay, 200*time.Millisecond)
	s.GreaterOrEqual(delay, time.Duration(0))

	// attempt 2: [0, 400ms]
	st = st.Report(errors.New("err"))
	delay, ok = st.Next()
	s.True(ok)
	s.LessOrEqual(delay, 400*time.Millisecond)
	s.GreaterOrEqual(delay, time.Duration(0))
}

func (s *StrategyTestSuite) TestExponentialStrategy_MaxDelayCap() {
	st := NewExponentialStrategy(1*time.Second, 5*time.Second, 10)

	for range 5 {
		st = st.Report(errors.New("err"))
	}

	delay, ok := st.Next()
	s.True(ok)
	s.LessOrEqual(delay, 5*time.Second)
}

func (s *StrategyTestSuite) TestExponentialStrategy_Exhausted() {
	st := NewExponentialStrategy(100*time.Millisecond, 10*time.Second, 2)

	st = st.Report(errors.New("err"))
	st = st.Report(errors.New("err"))

	_, ok := st.Next()
	s.False(ok)
}

func (s *StrategyTestSuite) TestExponentialStrategy_Immutable() {
	st := NewExponentialStrategy(100*time.Millisecond, 10*time.Second, 3)
	st2 := st.Report(errors.New("err"))

	d1, _ := st.Next()
	d2, _ := st2.Next()

	// attempt 0: [0, 100ms], attempt 1: [0, 200ms] (full jitter)
	s.LessOrEqual(d1, 100*time.Millisecond)
	s.LessOrEqual(d2, 200*time.Millisecond)
}

func (s *StrategyTestSuite) TestAdaptiveStrategy_StopsOnThreshold() {
	base := NewFixedStrategy(100*time.Millisecond, 100)
	st := NewAdaptiveStrategy(base, 5, 3)

	st = st.Report(errors.New("err1"))
	st = st.Report(errors.New("err2"))
	st = st.Report(errors.New("err3"))

	_, ok := st.Next()
	s.False(ok)
}

func (s *StrategyTestSuite) TestAdaptiveStrategy_ContinuesBelowThreshold() {
	base := NewFixedStrategy(100*time.Millisecond, 100)
	st := NewAdaptiveStrategy(base, 5, 3)

	st = st.Report(errors.New("err1"))
	st = st.Report(nil)
	st = st.Report(errors.New("err2"))

	_, ok := st.Next()
	s.True(ok)
}

func (s *StrategyTestSuite) TestAdaptiveStrategy_SlidingWindow() {
	base := NewFixedStrategy(100*time.Millisecond, 100)
	st := NewAdaptiveStrategy(base, 3, 3) // 窗口大小 3，阈值 3

	st = st.Report(errors.New("err"))
	st = st.Report(errors.New("err"))
	st = st.Report(errors.New("err"))

	_, ok := st.Next()
	s.False(ok)

	st = st.Report(nil)

	_, ok = st.Next()
	s.True(ok)
}

func (s *StrategyTestSuite) TestRetryWithStrategy_Success() {
	ctx := s.T().Context()
	st := NewFixedStrategy(time.Millisecond, 3)

	var count atomic.Int32
	err := RetryWithStrategy(ctx, st, func() error {
		count.Add(1)
		return nil
	})

	s.NoError(err)
	s.Equal(int32(1), count.Load())
}

func (s *StrategyTestSuite) TestRetryWithStrategy_EventualSuccess() {
	ctx := s.T().Context()
	st := NewFixedStrategy(time.Millisecond, 5)

	var count atomic.Int32
	err := RetryWithStrategy(ctx, st, func() error {
		if count.Add(1) < 3 {
			return errors.New("not yet")
		}
		return nil
	})

	s.NoError(err)
	s.Equal(int32(3), count.Load())
}

func (s *StrategyTestSuite) TestRetryWithStrategy_AllFail() {
	ctx := s.T().Context()
	st := NewFixedStrategy(time.Millisecond, 3)

	errFail := errors.New("persistent failure")
	var count atomic.Int32
	err := RetryWithStrategy(ctx, st, func() error {
		count.Add(1)
		return errFail
	})

	s.ErrorIs(err, errFail)
	// 执行 1 次 → Report(0→1) → Next(1<3) → 执行 2 次 → Report(1→2) → Next(2<3) → 执行 3 次 → Report(2→3) → Next(3=3,false) → 返回
	s.Equal(int32(3), count.Load())
}

func (s *StrategyTestSuite) TestRetryWithStrategy_ContextCanceled() {
	ctx, cancel := context.WithCancel(s.T().Context())
	cancel()

	st := NewFixedStrategy(time.Second, 10)
	err := RetryWithStrategy(ctx, st, func() error {
		return errors.New("should not run much")
	})

	s.ErrorIs(err, context.Canceled)
}

func (s *StrategyTestSuite) TestRetryWithStrategy_ContextTimeout() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 50*time.Millisecond)
	defer cancel()

	st := NewFixedStrategy(100*time.Millisecond, 100)
	err := RetryWithStrategy(ctx, st, func() error {
		return errors.New("keep failing")
	})

	s.Error(err)
}

func (s *StrategyTestSuite) TestRetryWithStrategy_ExponentialBackoff() {
	ctx := s.T().Context()
	st := NewExponentialStrategy(time.Millisecond, 100*time.Millisecond, 3)

	var count atomic.Int32
	err := RetryWithStrategy(ctx, st, func() error {
		if count.Add(1) < 3 {
			return errors.New("not yet")
		}
		return nil
	})

	s.NoError(err)
	s.Equal(int32(3), count.Load())
}

func (s *StrategyTestSuite) TestRetryWithStrategy_ZeroRetries() {
	ctx := s.T().Context()
	st := NewFixedStrategy(time.Millisecond, 0)

	errFail := errors.New("fail")
	err := RetryWithStrategy(ctx, st, func() error {
		return errFail
	})

	s.ErrorIs(err, errFail)
}
