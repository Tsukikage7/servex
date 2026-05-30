package delayqueue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type DelayQueueTestSuite struct {
	suite.Suite
}

func TestDelayQueueSuite(t *testing.T) {
	suite.Run(t, new(DelayQueueTestSuite))
}

type delayItem struct {
	value    string
	deadline time.Time
}

func (d delayItem) Delay() time.Duration {
	return time.Until(d.deadline)
}

func newItem(value string, delay time.Duration) delayItem {
	return delayItem{
		value:    value,
		deadline: time.Now().Add(delay),
	}
}

func (s *DelayQueueTestSuite) TestNew() {
	dq := New[delayItem]()
	s.NotNil(dq)
	s.True(dq.IsEmpty())
	s.Equal(0, dq.Len())
}

func (s *DelayQueueTestSuite) TestEnqueueDequeue_Expired() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	item := newItem("expired", -time.Second)
	s.NoError(dq.Enqueue(ctx, item))

	result, err := dq.Dequeue(ctx)
	s.NoError(err)
	s.Equal("expired", result.value)
}

func (s *DelayQueueTestSuite) TestEnqueueDequeue_WithDelay() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	start := time.Now()
	s.NoError(dq.Enqueue(ctx, newItem("delayed", 50*time.Millisecond)))

	result, err := dq.Dequeue(ctx)
	s.NoError(err)
	s.Equal("delayed", result.value)

	s.GreaterOrEqual(time.Since(start), 40*time.Millisecond)
}

func (s *DelayQueueTestSuite) TestDequeue_EmptyBlocks() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 50*time.Millisecond)
	defer cancel()

	dq := New[delayItem]()
	_, err := dq.Dequeue(ctx)
	s.ErrorIs(err, context.DeadlineExceeded)
}

func (s *DelayQueueTestSuite) TestDequeue_ContextCanceled() {
	ctx, cancel := context.WithCancel(s.T().Context())
	cancel()

	dq := New[delayItem]()
	_, err := dq.Dequeue(ctx)
	s.ErrorIs(err, context.Canceled)
}

func (s *DelayQueueTestSuite) TestEnqueue_ContextCanceled() {
	ctx, cancel := context.WithCancel(s.T().Context())
	cancel()

	dq := New[delayItem]()
	err := dq.Enqueue(ctx, newItem("item", time.Second))
	s.ErrorIs(err, context.Canceled)
}

func (s *DelayQueueTestSuite) TestEarlierElementWakesDequeue() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	s.NoError(dq.Enqueue(ctx, newItem("late", 5*time.Second)))

	done := make(chan string)
	go func() {
		result, err := dq.Dequeue(ctx)
		s.NoError(err)
		done <- result.value
	}()

	time.Sleep(20 * time.Millisecond)
	s.NoError(dq.Enqueue(ctx, newItem("early", 30*time.Millisecond)))

	select {
	case val := <-done:
		s.Equal("early", val)
	case <-time.After(2 * time.Second):
		s.Fail("Dequeue 未在超时前返回")
	}
}

func (s *DelayQueueTestSuite) TestOrderByDeadline() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	now := time.Now()
	s.NoError(dq.Enqueue(ctx, delayItem{"third", now.Add(60 * time.Millisecond)}))
	s.NoError(dq.Enqueue(ctx, delayItem{"first", now.Add(20 * time.Millisecond)}))
	s.NoError(dq.Enqueue(ctx, delayItem{"second", now.Add(40 * time.Millisecond)}))

	r1, err := dq.Dequeue(ctx)
	s.NoError(err)
	s.Equal("first", r1.value)

	r2, err := dq.Dequeue(ctx)
	s.NoError(err)
	s.Equal("second", r2.value)

	r3, err := dq.Dequeue(ctx)
	s.NoError(err)
	s.Equal("third", r3.value)
}

func (s *DelayQueueTestSuite) TestLen() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	s.NoError(dq.Enqueue(ctx, newItem("a", time.Hour)))
	s.NoError(dq.Enqueue(ctx, newItem("b", time.Hour)))
	s.Equal(2, dq.Len())
	s.False(dq.IsEmpty())
}

func (s *DelayQueueTestSuite) TestConcurrentEnqueueDequeue() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	const numItems = 50
	var wg sync.WaitGroup

	for i := range numItems {
		wg.Go(func() {
			s.NoError(dq.Enqueue(ctx, newItem("item", -time.Duration(i)*time.Millisecond)))
		})
	}
	wg.Wait()

	results := make(chan string, numItems)
	for range numItems {
		wg.Go(func() {
			result, err := dq.Dequeue(ctx)
			s.NoError(err)
			results <- result.value
		})
	}
	wg.Wait()
	close(results)

	count := 0
	for range results {
		count++
	}
	s.Equal(numItems, count)
}

func (s *DelayQueueTestSuite) TestAll() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	now := time.Now()
	s.NoError(dq.Enqueue(ctx, delayItem{"third", now.Add(30 * time.Millisecond)}))
	s.NoError(dq.Enqueue(ctx, delayItem{"first", now.Add(10 * time.Millisecond)}))
	s.NoError(dq.Enqueue(ctx, delayItem{"second", now.Add(20 * time.Millisecond)}))

	// 按到期时间顺序遍历
	var values []string
	for item := range dq.All() {
		values = append(values, item.value)
	}
	s.Equal([]string{"first", "second", "third"}, values)
	// 原队列不受影响
	s.Equal(3, dq.Len())
}

func (s *DelayQueueTestSuite) TestAllEmpty() {
	dq := New[delayItem]()
	count := 0
	for range dq.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *DelayQueueTestSuite) TestAllEarlyBreak() {
	ctx := s.T().Context()
	dq := New[delayItem]()

	now := time.Now()
	s.NoError(dq.Enqueue(ctx, delayItem{"a", now.Add(10 * time.Millisecond)}))
	s.NoError(dq.Enqueue(ctx, delayItem{"b", now.Add(20 * time.Millisecond)}))
	s.NoError(dq.Enqueue(ctx, delayItem{"c", now.Add(30 * time.Millisecond)}))

	count := 0
	for range dq.All() {
		count++
		if count == 2 {
			break
		}
	}
	s.Equal(2, count)
	s.Equal(3, dq.Len())
}
