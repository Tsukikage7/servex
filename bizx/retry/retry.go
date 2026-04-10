// Package retry 提供异步持久化重试机制，支持指数退避.
package retry

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNilStore 存储为空.
	ErrNilStore = errors.New("retry: store is nil")
	// ErrHandlerNotFound 处理器未注册.
	ErrHandlerNotFound = errors.New("retry: handler not found")
)

// Status 任务状态.
type Status string

const (
	// StatusPending 待处理状态.
	StatusPending Status = "pending"
	// StatusRunning 运行中状态.
	StatusRunning Status = "running"
	// StatusDone 已完成状态.
	StatusDone Status = "done"
	// StatusDead 已失败状态.
	StatusDead Status = "dead"
)

// Task 重试任务.
type Task struct {
	ID                string          `json:"id" gorm:"primaryKey"`
	Name              string          `json:"name" gorm:"index"`
	Payload           json.RawMessage `json:"payload"`
	MaxRetries        int             `json:"max_retries"`
	Retried           int             `json:"retried"`
	NextRetryAt       time.Time       `json:"next_retry_at" gorm:"index"`
	Status            Status          `json:"status" gorm:"index"`
	LastError         string          `json:"last_error"`
	InitialDelay      time.Duration   `json:"initial_delay"`
	BackoffMultiplier float64         `json:"backoff_multiplier"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// Handler 任务处理函数.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Scheduler 重试调度器.
type Scheduler interface {
	Submit(ctx context.Context, name string, payload any, opts ...TaskOption) (string, error)
	Register(name string, handler Handler)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// TaskOption 任务选项.
type TaskOption func(*taskOptions)

type taskOptions struct {
	maxRetries        int
	initialDelay      time.Duration
	backoffMultiplier float64
}

// WithMaxRetries 设置最大重试次数，默认 5.
func WithMaxRetries(n int) TaskOption {
	return func(o *taskOptions) {
		o.maxRetries = n
	}
}

// WithInitialDelay 设置初始延迟，默认 1m.
func WithInitialDelay(d time.Duration) TaskOption {
	return func(o *taskOptions) {
		o.initialDelay = d
	}
}

// WithBackoffMultiplier 设置退避倍数，默认 2.0.
func WithBackoffMultiplier(m float64) TaskOption {
	return func(o *taskOptions) {
		o.backoffMultiplier = m
	}
}

// Store 任务存储接口.
type Store interface {
	Save(ctx context.Context, task *Task) error
	FetchPending(ctx context.Context, limit int) ([]Task, error)
	Update(ctx context.Context, task *Task) error
	AutoMigrate(ctx context.Context) error
}

// Option 调度器选项.
type Option func(*scheduler)

// WithPollInterval 设置轮询间隔，默认 10s.
func WithPollInterval(d time.Duration) Option {
	return func(s *scheduler) {
		s.pollInterval = d
	}
}

// WithConcurrency 设置并发数，默认 5.
func WithConcurrency(n int) Option {
	return func(s *scheduler) {
		s.concurrency = n
	}
}

// scheduler 调度器实现.
type scheduler struct {
	store        Store
	handlers     map[string]Handler
	mu           sync.RWMutex
	pollInterval time.Duration
	concurrency  int
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	sem          chan struct{}
	startOnce    sync.Once
}

// NewScheduler 创建重试调度器.
func NewScheduler(store Store, opts ...Option) Scheduler {
	s := &scheduler{
		store:        store,
		handlers:     make(map[string]Handler),
		pollInterval: 10 * time.Second,
		concurrency:  5,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.sem = make(chan struct{}, s.concurrency)
	return s
}

// Register 注册任务处理器.
func (s *scheduler) Register(name string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = handler
}

// Submit 提交重试任务.
func (s *scheduler) Submit(ctx context.Context, name string, payload any, opts ...TaskOption) (string, error) {
	if s.store == nil {
		return "", ErrNilStore
	}

	o := taskOptions{
		maxRetries:        5,
		initialDelay:      time.Minute,
		backoffMultiplier: 2.0,
	}
	for _, opt := range opts {
		opt(&o)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	task := &Task{
		ID:                uuid.New().String(),
		Name:              name,
		Payload:           data,
		MaxRetries:        o.maxRetries,
		Retried:           0,
		NextRetryAt:       time.Now(),
		Status:            StatusPending,
		InitialDelay:      o.initialDelay,
		BackoffMultiplier: o.backoffMultiplier,
	}

	if err := s.store.Save(ctx, task); err != nil {
		return "", err
	}
	return task.ID, nil
}

// Start 启动调度器（多次调用仅首次生效）.
func (s *scheduler) Start(ctx context.Context) error {
	if s.store == nil {
		return ErrNilStore
	}

	var startErr error
	s.startOnce.Do(func() {
		ctx, s.cancel = context.WithCancel(ctx)
		s.wg.Add(1)
		go s.poll(ctx)
	})
	return startErr
}

// Stop 停止调度器.
func (s *scheduler) Stop(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

// poll 轮询待处理任务.
func (s *scheduler) poll(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processBatch(ctx)
		}
	}
}

// processBatch 处理一批待处理任务.
func (s *scheduler) processBatch(ctx context.Context) {
	tasks, err := s.store.FetchPending(ctx, s.concurrency)
	if err != nil {
		return
	}

	for _, task := range tasks {
		task := task
		s.sem <- struct{}{}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.sem }()
			s.processTask(ctx, &task)
		}()
	}
}

// processTask 处理单个任务.
func (s *scheduler) processTask(ctx context.Context, task *Task) {
	s.mu.RLock()
	handler, ok := s.handlers[task.Name]
	s.mu.RUnlock()

	if !ok {
		task.Status = StatusDead
		task.LastError = ErrHandlerNotFound.Error()
		_ = s.store.Update(ctx, task)
		return
	}

	task.Status = StatusRunning
	_ = s.store.Update(ctx, task)

	err := handler(ctx, task.Payload)
	if err == nil {
		task.Status = StatusDone
		_ = s.store.Update(ctx, task)
		return
	}

	task.Retried++
	task.LastError = err.Error()

	if task.Retried >= task.MaxRetries {
		task.Status = StatusDead
	} else {
		task.Status = StatusPending
		// 指数退避：使用任务自身的退避参数
		initialDelay := task.InitialDelay
		if initialDelay <= 0 {
			initialDelay = time.Minute
		}
		multiplier := task.BackoffMultiplier
		if multiplier <= 0 {
			multiplier = 2.0
		}
		delay := initialDelay * time.Duration(math.Pow(multiplier, float64(task.Retried-1)))
		task.NextRetryAt = time.Now().Add(delay)
	}
	_ = s.store.Update(ctx, task)
}

// --- Memory Store ---

type memoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewMemoryStore 创建基于内存的任务存储（用于测试）.
func NewMemoryStore() Store {
	return &memoryStore{tasks: make(map[string]*Task)}
}

func (s *memoryStore) Save(_ context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	s.tasks[task.ID] = &cp
	return nil
}

func (s *memoryStore) FetchPending(_ context.Context, limit int) ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var result []Task
	for _, t := range s.tasks {
		if t.Status == StatusPending && !t.NextRetryAt.After(now) {
			result = append(result, *t)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) Update(_ context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *task
	cp.UpdatedAt = time.Now()
	s.tasks[task.ID] = &cp
	return nil
}

func (s *memoryStore) AutoMigrate(_ context.Context) error {
	return nil
}
