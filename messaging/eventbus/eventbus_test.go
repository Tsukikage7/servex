package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEvent 测试用事件.
type testEvent struct {
	topic   string
	payload string
}

func (e testEvent) Topic() string { return e.topic }

func TestSubscribeAndPublish(t *testing.T) {
	bus := New()
	defer bus.Close()

	var received string
	bus.Subscribe("user.created", func(_ context.Context, e Event) error {
		received = e.(testEvent).payload
		return nil
	})

	err := bus.Publish(t.Context(), testEvent{topic: "user.created", payload: "alice"})
	require.NoError(t, err)
	assert.Equal(t, "alice", received)
}

func TestPublishNoSubscribers(t *testing.T) {
	bus := New()
	defer bus.Close()

	err := bus.Publish(t.Context(), testEvent{topic: "nothing", payload: "x"})
	assert.NoError(t, err)
}

func TestPublishHandlerError(t *testing.T) {
	bus := New()
	defer bus.Close()

	want := errors.New("handler failed")
	bus.Subscribe("fail", func(_ context.Context, _ Event) error {
		return want
	})

	err := bus.Publish(t.Context(), testEvent{topic: "fail"})
	assert.ErrorIs(t, err, want)
}

func TestMultipleSubscribers(t *testing.T) {
	bus := New()
	defer bus.Close()

	var count atomic.Int32
	for range 3 {
		bus.Subscribe("inc", func(_ context.Context, _ Event) error {
			count.Add(1)
			return nil
		})
	}

	err := bus.Publish(t.Context(), testEvent{topic: "inc"})
	require.NoError(t, err)
	assert.Equal(t, int32(3), count.Load())
}

func TestUnsubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	var count atomic.Int32
	unsub := bus.Subscribe("topic", func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	_ = bus.Publish(t.Context(), testEvent{topic: "topic"})
	assert.Equal(t, int32(1), count.Load())

	unsub()

	_ = bus.Publish(t.Context(), testEvent{topic: "topic"})
	assert.Equal(t, int32(1), count.Load())
}

func TestSubscribeAll(t *testing.T) {
	bus := New()
	defer bus.Close()

	var topics []string
	bus.SubscribeAll(func(_ context.Context, e Event) error {
		topics = append(topics, e.Topic())
		return nil
	})

	_ = bus.Publish(t.Context(), testEvent{topic: "a"})
	_ = bus.Publish(t.Context(), testEvent{topic: "b"})

	assert.Equal(t, []string{"a", "b"}, topics)
}

func TestSubscribeAllUnsubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	var count atomic.Int32
	unsub := bus.SubscribeAll(func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	_ = bus.Publish(t.Context(), testEvent{topic: "x"})
	assert.Equal(t, int32(1), count.Load())

	unsub()

	_ = bus.Publish(t.Context(), testEvent{topic: "y"})
	assert.Equal(t, int32(1), count.Load())
}

func TestPublishAsync(t *testing.T) {
	bus := New(WithAsyncWorkers(2))

	var count atomic.Int32
	bus.Subscribe("async", func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	for range 10 {
		bus.PublishAsync(t.Context(), testEvent{topic: "async"})
	}

	bus.Close()
	assert.Equal(t, int32(10), count.Load())
}

func TestPublishAsyncErrorHandler(t *testing.T) {
	var errCount atomic.Int32
	bus := New(
		WithAsyncWorkers(1),
		WithErrorHandler(func(_ error) {
			errCount.Add(1)
		}),
	)

	bus.Subscribe("err", func(_ context.Context, _ Event) error {
		return errors.New("boom")
	})

	bus.PublishAsync(t.Context(), testEvent{topic: "err"})
	bus.Close()

	assert.Equal(t, int32(1), errCount.Load())
}

func TestClosedBusPublish(t *testing.T) {
	bus := New()
	bus.Close()

	err := bus.Publish(t.Context(), testEvent{topic: "x"})
	assert.ErrorIs(t, err, ErrBusClosed)
}

func TestClosedBusPublishAsync(t *testing.T) {
	bus := New()
	bus.Close()

	// 不应 panic.
	bus.PublishAsync(t.Context(), testEvent{topic: "x"})
}

func TestDoubleClose(t *testing.T) {
	bus := New()
	bus.Close()
	bus.Close() // 不应 panic.
}

func TestConcurrentSafety(t *testing.T) {
	bus := New(WithAsyncWorkers(4))

	var wg sync.WaitGroup
	var count atomic.Int32

	// 并发订阅.
	for i := range 10 {
		wg.Go(func() {
			unsub := bus.Subscribe("concurrent", func(_ context.Context, _ Event) error {
				count.Add(1)
				return nil
			})
			// 部分取消订阅.
			if i%2 == 0 {
				unsub()
			}
		})
	}
	wg.Wait()

	// 并发发布.
	for range 20 {
		wg.Go(func() {
			_ = bus.Publish(t.Context(), testEvent{topic: "concurrent"})
		})
	}
	wg.Wait()

	bus.Close()
	assert.Greater(t, count.Load(), int32(0))
}

func TestWithLogger(t *testing.T) {
	// 仅验证选项不 panic.
	bus := New(WithLogger(nil))
	bus.Close()
}

func TestWithAsyncWorkersZero(t *testing.T) {
	// n <= 0 应使用默认值.
	bus := New(WithAsyncWorkers(0))

	var called atomic.Bool
	bus.Subscribe("test", func(_ context.Context, _ Event) error {
		called.Store(true)
		return nil
	})
	bus.PublishAsync(t.Context(), testEvent{topic: "test"})
	bus.Close()
	assert.True(t, called.Load())
}

func TestWildcardAndTopicCombined(t *testing.T) {
	bus := New()
	defer bus.Close()

	var results []string
	bus.Subscribe("order", func(_ context.Context, e Event) error {
		results = append(results, "topic:"+e.Topic())
		return nil
	})
	bus.SubscribeAll(func(_ context.Context, e Event) error {
		results = append(results, "wildcard:"+e.Topic())
		return nil
	})

	_ = bus.Publish(t.Context(), testEvent{topic: "order"})
	assert.Equal(t, []string{"topic:order", "wildcard:order"}, results)
}

func TestPublishAsyncClose(t *testing.T) {
	bus := New(WithAsyncWorkers(2))

	var count atomic.Int32
	bus.Subscribe("slow", func(_ context.Context, _ Event) error {
		time.Sleep(10 * time.Millisecond)
		count.Add(1)
		return nil
	})

	for range 5 {
		bus.PublishAsync(t.Context(), testEvent{topic: "slow"})
	}

	bus.Close() // 应等待所有异步任务完成.
	assert.Equal(t, int32(5), count.Load())
}
