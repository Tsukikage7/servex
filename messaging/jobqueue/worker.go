package jobqueue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type worker struct {
	store    Store
	handlers map[string]Handler
	mu       sync.RWMutex
	closed   atomic.Bool
	opts     workerOptions
}

// NewWorker 创建任务消费 Worker.
func NewWorker(store Store, opts ...WorkerOption) Worker {
	o := workerOptions{
		concurrency:  1,
		pollInterval: time.Second,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &worker{
		store:    store,
		handlers: make(map[string]Handler),
		opts:     o,
	}
}

func (w *worker) Register(jobType string, handler Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[jobType] = handler
}

// Start 启动 Worker，阻塞直到 ctx 取消.
func (w *worker) Start(ctx context.Context) error {
	if len(w.opts.queues) == 0 {
		return ErrNoQueues
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, w.opts.concurrency)

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		default:
		}

		// 检查是否已关闭，阻止接收新任务
		if w.closed.Load() {
			wg.Wait()
			return nil
		}

		job := w.fetchJob(ctx)
		if job == nil {
			select {
			case <-ctx.Done():
				wg.Wait()
				return nil
			case <-time.After(w.opts.pollInterval):
				continue
			}
		}

		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			w.processJob(ctx, job)
		})
	}
}

func (w *worker) fetchJob(ctx context.Context) *Job {
	for _, queue := range w.opts.queues {
		job, err := w.store.Dequeue(ctx, queue)
		if err != nil {
			continue
		}
		return job
	}
	return nil
}

func (w *worker) processJob(ctx context.Context, job *Job) {
	if err := w.store.MarkRunning(ctx, job.ID); err != nil {
		w.logErrorf("标记任务为运行中失败 [id:%s]: %v", job.ID, err)
	}

	w.mu.RLock()
	handler, ok := w.handlers[job.Type]
	w.mu.RUnlock()

	if !ok {
		if err := w.store.MarkFailed(ctx, job.ID, ErrNoHandler); err != nil {
			w.logErrorf("标记任务为失败失败 [id:%s]: %v", job.ID, err)
		}
		return
	}

	jobCtx := ctx
	if !job.Deadline.IsZero() {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithDeadline(ctx, job.Deadline)
		defer cancel()
	}

	if err := handler(jobCtx, job); err != nil {
		job.Retried++
		job.LastError = err.Error()
		if job.Retried >= job.MaxRetries {
			if markErr := w.store.MarkDead(ctx, job.ID); markErr != nil {
				w.logErrorf("标记任务为死信失败 [id:%s]: %v", job.ID, markErr)
			}
		} else {
			job.Status = StatusPending
			job.ScheduledAt = time.Now().Add(w.backoff(job.Retried))
			if requeueErr := w.store.Requeue(ctx, job); requeueErr != nil {
				w.logErrorf("重新入队任务失败 [id:%s]: %v", job.ID, requeueErr)
			}
		}
		return
	}

	// MarkDone 带重试（最多 3 次指数退避），避免任务执行成功但状态未更新
	w.markDoneWithRetry(ctx, job.ID)
}

// markDoneWithRetry 带重试的 MarkDone（最多 3 次，指数退避）.
func (w *worker) markDoneWithRetry(ctx context.Context, id string) {
	const maxAttempts = 3
	backoff := 100 * time.Millisecond
	for i := range maxAttempts {
		if err := w.store.MarkDone(ctx, id); err != nil {
			w.logErrorf("标记任务完成失败 [id:%s attempt:%d]: %v", id, i+1, err)
			if i < maxAttempts-1 {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}
		return
	}
}

func (w *worker) logErrorf(format string, args ...any) {
	if w.opts.logger != nil {
		w.opts.logger.Errorf("[JobQueue] "+format, args...)
	}
}

func (w *worker) backoff(retried int) time.Duration {
	d := 50 * time.Millisecond
	for range retried {
		d *= 2
	}
	if d > 5*time.Minute {
		d = 5 * time.Minute
	}
	return d
}

func (w *worker) Close() error {
	w.closed.Store(true)
	return nil
}
