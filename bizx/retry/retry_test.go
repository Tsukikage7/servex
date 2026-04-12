package retry

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmit(t *testing.T) {
	store := NewMemoryStore()
	s := NewScheduler(store)

	id, err := s.Submit(t.Context(), "send-email", map[string]string{"to": "alice@example.com"})
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// 验证任务已保存
	tasks, err := store.FetchPending(t.Context(), 10)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "send-email", tasks[0].Name)
	assert.Equal(t, StatusPending, tasks[0].Status)
}

func TestScheduler_Process(t *testing.T) {
	store := NewMemoryStore()
	s := NewScheduler(store, WithPollInterval(50*time.Millisecond), WithConcurrency(2))

	var processed atomic.Int32
	s.Register("test-task", func(ctx context.Context, payload json.RawMessage) error {
		processed.Add(1)
		return nil
	})

	_, err := s.Submit(t.Context(), "test-task", "hello")
	require.NoError(t, err)

	err = s.Start(t.Context())
	require.NoError(t, err)

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	err = s.Stop(t.Context())
	require.NoError(t, err)

	assert.Equal(t, int32(1), processed.Load())
}

func TestRetry_Backoff(t *testing.T) {
	store := NewMemoryStore()
	s := NewScheduler(store, WithPollInterval(50*time.Millisecond))

	var attempts atomic.Int32
	s.Register("flaky-task", func(ctx context.Context, payload json.RawMessage) error {
		n := attempts.Add(1)
		if n < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	_, err := s.Submit(t.Context(), "flaky-task", "data",
		WithMaxRetries(5),
	)
	require.NoError(t, err)

	// 手动处理几轮，因为退避延迟会导致自动轮询太慢
	sched := s.(*scheduler)
	ctx := t.Context()

	// 第一次处理 - 失败
	sched.processBatch(ctx)
	time.Sleep(50 * time.Millisecond)

	// 修改 NextRetryAt 让它立即可被拉取
	ms := store.(*memoryStore)
	ms.mu.Lock()
	for _, task := range ms.tasks {
		task.NextRetryAt = time.Now().Add(-time.Second)
	}
	ms.mu.Unlock()

	// 第二次处理 - 失败
	sched.processBatch(ctx)
	time.Sleep(50 * time.Millisecond)

	ms.mu.Lock()
	for _, task := range ms.tasks {
		task.NextRetryAt = time.Now().Add(-time.Second)
	}
	ms.mu.Unlock()

	// 第三次处理 - 成功
	sched.processBatch(ctx)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(3), attempts.Load())

	// 验证任务状态为完成
	ms.mu.RLock()
	for _, task := range ms.tasks {
		assert.Equal(t, StatusDone, task.Status)
	}
	ms.mu.RUnlock()
}

func TestMaxRetries_Dead(t *testing.T) {
	store := NewMemoryStore()
	s := NewScheduler(store, WithPollInterval(50*time.Millisecond))

	s.Register("always-fail", func(ctx context.Context, payload json.RawMessage) error {
		return errors.New("always fails")
	})

	_, err := s.Submit(t.Context(), "always-fail", "data",
		WithMaxRetries(2),
	)
	require.NoError(t, err)

	sched := s.(*scheduler)
	ctx := t.Context()
	ms := store.(*memoryStore)

	// 处理直到超过最大重试次数
	for i := 0; i < 3; i++ {
		sched.processBatch(ctx)
		time.Sleep(50 * time.Millisecond)
		// 重置 NextRetryAt
		ms.mu.Lock()
		for _, task := range ms.tasks {
			task.NextRetryAt = time.Now().Add(-time.Second)
		}
		ms.mu.Unlock()
	}

	// 验证任务状态为死亡
	ms.mu.RLock()
	for _, task := range ms.tasks {
		assert.Equal(t, StatusDead, task.Status)
		assert.Equal(t, "always fails", task.LastError)
	}
	ms.mu.RUnlock()
}

func TestRegisterHandler(t *testing.T) {
	store := NewMemoryStore()
	s := NewScheduler(store, WithPollInterval(50*time.Millisecond))

	// 提交任务但不注册处理器
	_, err := s.Submit(t.Context(), "unknown-task", "data")
	require.NoError(t, err)

	sched := s.(*scheduler)
	sched.processBatch(t.Context())
	time.Sleep(50 * time.Millisecond)

	// 验证任务标记为 dead
	ms := store.(*memoryStore)
	ms.mu.RLock()
	for _, task := range ms.tasks {
		assert.Equal(t, StatusDead, task.Status)
		assert.Contains(t, task.LastError, "handler not found")
	}
	ms.mu.RUnlock()
}
