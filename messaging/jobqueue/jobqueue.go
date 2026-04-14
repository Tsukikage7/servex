// Package jobqueue 提供异步任务队列的核心抽象与实现.
package jobqueue

import (
	"context"
	"time"
)

// Status 表示 Job 的执行状态.
type Status string

const (
	// StatusPending 表示任务等待执行.
	StatusPending Status = "pending"
	// StatusRunning 表示任务正在执行.
	StatusRunning Status = "running"
	// StatusFailed 表示任务执行失败.
	StatusFailed Status = "failed"
	// StatusDead 表示任务已达到最大重试次数.
	StatusDead Status = "dead"
)

// Job 表示一个待执行的异步任务.
type Job struct {
	ID          string
	Queue       string
	Type        string
	Payload     []byte
	Headers     map[string]string
	Priority    int
	MaxRetries  int
	Retried     int
	Delay       time.Duration
	Deadline    time.Time
	Status      Status
	LastError   string
	CreatedAt   time.Time
	ScheduledAt time.Time
}

// Handler 处理特定类型的任务.
type Handler func(ctx context.Context, job *Job) error

// Client 负责投递任务.
type Client interface {
	Enqueue(ctx context.Context, job *Job) error
	Close() error
}

// Worker 负责拉取并执行任务.
type Worker interface {
	Register(jobType string, handler Handler)
	Start(ctx context.Context) error
	Close() error
}

// Store 是任务存储后端的抽象.
type Store interface {
	Enqueue(ctx context.Context, job *Job) error
	Dequeue(ctx context.Context, queue string) (*Job, error)
	MarkRunning(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, err error) error
	MarkDead(ctx context.Context, id string) error
	MarkDone(ctx context.Context, id string) error
	Requeue(ctx context.Context, job *Job) error
	ListDead(ctx context.Context, queue string) ([]*Job, error)
	Close() error
}
