package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Tsukikage7/servex/observability/logger"
)

// cronScheduler 基于 Cron 的调度器实现.
type cronScheduler struct {
	cron    *cron.Cron
	jobs    map[string]*Job
	opts    *options
	mu      sync.RWMutex
	running bool
	closed  bool
	wg      sync.WaitGroup // 跟踪正在执行的任务
}

// newCronScheduler 创建 Cron 调度器.
func newCronScheduler(opts ...Option) (*cronScheduler, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	var cronOpts []cron.Option
	if o.withSeconds {
		cronOpts = append(cronOpts, cron.WithSeconds())
	}
	if o.location != nil {
		cronOpts = append(cronOpts, cron.WithLocation(o.location))
	}

	return &cronScheduler{
		cron: cron.New(cronOpts...),
		jobs: make(map[string]*Job),
		opts: o,
	}, nil
}

// Add 添加任务.
func (s *cronScheduler) Add(job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSchedulerClosed
	}

	if _, exists := s.jobs[job.Name]; exists {
		return ErrJobExists
	}

	// 设置默认超时
	if job.Timeout == 0 {
		job.Timeout = s.opts.defaultTimeout
	}

	// 初始化统计
	job.initStats()

	// 如果已在运行，立即注册
	if s.running {
		if err := s.registerJob(job); err != nil {
			return err
		}
	}

	s.jobs[job.Name] = job
	s.logDebugf("任务已添加: %s [schedule:%s, singleton:%v, distributed:%v]",
		job.Name, job.Schedule, job.Singleton, job.Distributed)

	return nil
}

// Remove 移除任务.
func (s *cronScheduler) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[name]
	if !exists {
		return ErrJobNotFound
	}

	if s.running && job.entryID > 0 {
		s.cron.Remove(cron.EntryID(job.entryID))
	}

	delete(s.jobs, name)
	s.logDebugf("任务已移除: %s", name)

	return nil
}

// Get 获取任务.
func (s *cronScheduler) Get(name string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[name]
	return job, exists
}

// List 列出所有任务.
func (s *cronScheduler) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// Start 启动调度器.
func (s *cronScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSchedulerClosed
	}

	if s.running {
		return nil
	}

	// 注册所有任务
	for _, job := range s.jobs {
		if err := s.registerJob(job); err != nil {
			return err
		}
	}

	s.cron.Start()
	s.running = true

	s.logDebug("调度器已启动")
	for _, job := range s.jobs {
		s.logDebugf("已注册: %s [schedule:%s]", job.Name, job.Schedule)
	}

	return nil
}

// Stop 停止调度器.
func (s *cronScheduler) Stop() {
	s.mu.Lock()
	if !s.running || s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	s.logDebug("调度器已停止")
}

// Shutdown 优雅关闭.
func (s *cronScheduler) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// 停止接受新任务
	cronCtx := s.cron.Stop()

	// 等待正在执行的任务完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-cronCtx.Done():
		// cron 已停止
	case <-ctx.Done():
		s.logWarn("调度器关闭超时")
		return ctx.Err()
	}

	select {
	case <-done:
		s.logDebug("调度器优雅关闭完成")
	case <-ctx.Done():
		s.logWarn("等待任务完成超时")
		return ctx.Err()
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	return nil
}

// Running 检查是否运行中.
func (s *cronScheduler) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Trigger 立即触发任务执行.
func (s *cronScheduler) Trigger(name string) error {
	s.mu.RLock()
	job, exists := s.jobs[name]
	closed := s.closed
	s.mu.RUnlock()

	if closed {
		return ErrSchedulerClosed
	}
	if !exists {
		return ErrJobNotFound
	}

	s.wg.Go(func() { s.executeJob(job) })
	return nil
}

// registerJob 注册任务到 cron.
func (s *cronScheduler) registerJob(job *Job) error {
	j := job // 避免闭包问题
	entryID, err := s.cron.AddFunc(j.Schedule, func() {
		s.wg.Go(func() { s.executeJob(j) })
	})
	if err != nil {
		return ErrScheduleInvalid
	}
	j.entryID = int(entryID)
	return nil
}

// executeJob 执行任务.
func (s *cronScheduler) executeJob(job *Job) {

	ctx := context.Background()
	jc := &JobContext{
		Job:       job,
		StartTime: time.Now(),
		Attempt:   1,
	}

	// 1. 单例检查（本地）
	if job.Singleton {
		if !job.tryStart() {
			jc.Skipped = true
			jc.SkipReason = "previous execution still running"
			job.stats.recordSkip()
			s.opts.hooks.runSkipHooks(ctx, jc)
			s.logDebugf("任务跳过（单例模式）: %s", job.Name)
			return
		}
		defer job.finish()
	}

	// 2. 分布式锁检查
	if job.Distributed && s.opts.locker != nil {
		lockKey := job.Name
		lockTTL := s.opts.lockTTL
		if job.Timeout > 0 && job.Timeout > lockTTL {
			lockTTL = job.Timeout + time.Minute // 锁时间略大于任务超时
		}

		acquired, err := s.opts.locker.TryLock(ctx, lockKey, lockTTL)
		if err != nil {
			s.logErrorf("获取分布式锁失败 [job:%s] [error:%v]", job.Name, err)
			jc.Skipped = true
			jc.SkipReason = "failed to acquire distributed lock"
			jc.Error = err
			job.stats.recordSkip()
			s.opts.hooks.runSkipHooks(ctx, jc)
			return
		}
		if !acquired {
			jc.Skipped = true
			jc.SkipReason = "distributed lock held by another instance"
			job.stats.recordSkip()
			s.opts.hooks.runSkipHooks(ctx, jc)
			s.logDebugf("任务跳过（分布式锁）: %s", job.Name)
			return
		}
		defer func() {
			if err := s.opts.locker.Unlock(ctx, lockKey); err != nil {
				s.logErrorf("释放分布式锁失败 [job:%s] [error:%v]", job.Name, err)
			}
		}()
	}

	// 3. 执行前置钩子
	if err := s.opts.hooks.runBeforeHooks(ctx, jc); err != nil {
		s.logDebugf("前置钩子阻止任务执行 [job:%s] [error:%v]", job.Name, err)
		return
	}

	// 4. 执行任务（带重试）
	s.runWithRetry(ctx, job, jc)
}

// runWithRetry 执行任务（带重试）.
func (s *cronScheduler) runWithRetry(ctx context.Context, job *Job, jc *JobContext) {
	maxAttempts := job.RetryCount + 1

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		jc.Attempt = attempt

		// 创建超时上下文
		execCtx, cancel := context.WithTimeout(ctx, job.Timeout)

		start := time.Now()
		job.stats.recordStart()

		s.logDebugf("开始执行任务: %s [attempt:%d/%d]", job.Name, attempt, maxAttempts)

		err := job.Handler(execCtx)
		duration := time.Since(start)
		cancel()

		jc.Duration = duration
		jc.Error = err

		if err == nil {
			job.stats.recordSuccess(duration)
			s.opts.hooks.runAfterHooks(ctx, jc)
			s.logDebugf("任务执行成功: %s [duration:%v]", job.Name, duration)
			return
		}

		job.stats.recordFail(duration, err)
		s.logErrorf("任务执行失败: %s [attempt:%d/%d] [error:%v]", job.Name, attempt, maxAttempts, err)

		// 最后一次尝试失败
		if attempt >= maxAttempts {
			s.opts.hooks.runErrorHooks(ctx, jc)
			s.opts.hooks.runAfterHooks(ctx, jc)
			return
		}

		// 等待重试间隔（尊重 context 取消）
		if job.RetryInterval > 0 {
			retryTimer := time.NewTimer(job.RetryInterval)
			select {
			case <-ctx.Done():
				retryTimer.Stop()
				s.opts.hooks.runErrorHooks(ctx, jc)
				s.opts.hooks.runAfterHooks(ctx, jc)
				return
			case <-retryTimer.C:
			}
		}
	}
}

// 日志辅助方法.

func (s *cronScheduler) logger() logger.Logger {
	return s.opts.logger
}

func (s *cronScheduler) logDebug(msg string) {
	if log := s.logger(); log != nil {
		log.Debug("[Scheduler] " + msg)
	}
}

func (s *cronScheduler) logDebugf(format string, args ...any) {
	if log := s.logger(); log != nil {
		log.Debugf("[Scheduler] "+format, args...)
	}
}

func (s *cronScheduler) logWarn(msg string) {
	if log := s.logger(); log != nil {
		log.Warn("[Scheduler] " + msg)
	}
}

func (s *cronScheduler) logErrorf(format string, args ...any) {
	if log := s.logger(); log != nil {
		log.Errorf("[Scheduler] "+format, args...)
	}
}
