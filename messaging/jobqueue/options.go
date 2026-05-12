package jobqueue

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

type workerOptions struct {
	queues       []string
	concurrency  int
	pollInterval time.Duration
	logger       logger.Logger
}

// WorkerOption 配置 Worker.
type WorkerOption func(*workerOptions)

// WithQueues 设置 Worker 要消费的队列列表.
func WithQueues(queues ...string) WorkerOption {
	return func(o *workerOptions) { o.queues = queues }
}

// WithConcurrency 设置 Worker 的并发数.
func WithConcurrency(n int) WorkerOption {
	return func(o *workerOptions) { o.concurrency = n }
}

// WithPollInterval 设置 Worker 的轮询间隔.
func WithPollInterval(d time.Duration) WorkerOption {
	return func(o *workerOptions) { o.pollInterval = d }
}

// WithLogger 设置 Worker 的日志器.
func WithLogger(log logger.Logger) WorkerOption {
	return func(o *workerOptions) { o.logger = logger.WithComponent(log, "JobQueue") }
}
