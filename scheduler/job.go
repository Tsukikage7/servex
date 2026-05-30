package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// JobFunc 任务执行函数.
type JobFunc func(ctx context.Context) error

// JobState 任务状态.
type JobState int32

const (
	// JobStateIdle 空闲状态.
	JobStateIdle JobState = iota
	// JobStateRunning 执行中.
	JobStateRunning
	// JobStatePaused 已暂停.
	JobStatePaused
)

// String 返回状态字符串.
func (s JobState) String() string {
	switch s {
	case JobStateIdle:
		return "idle"
	case JobStateRunning:
		return "running"
	case JobStatePaused:
		return "paused"
	default:
		return "unknown"
	}
}

// Job 调度任务.
type Job struct {
	// Name 任务名称唯一标识.
	Name string

	// Schedule Cron 表达式.
	Schedule string

	// Handler 任务处理函数.
	Handler JobFunc

	// Timeout 任务超时时间0 表示使用调度器默认值.
	Timeout time.Duration

	// Singleton 单例模式，防止任务重叠执行.
	// 如果上一次执行未完成，跳过本次调度.
	Singleton bool

	// Distributed 分布式模式，多实例部署时只有一个实例执行.
	// 需要配合 Locker 使用.
	Distributed bool

	// RetryCount 失败重试次数0 表示不重试.
	RetryCount int

	// RetryInterval 重试间隔.
	RetryInterval time.Duration

	// internal fields
	entryID   int
	state     atomic.Int32
	stats     *JobStats
	statsOnce sync.Once
}

// JobStats 任务执行统计.
type JobStats struct {
	mu            sync.RWMutex
	RunCount      int64         // 执行次数
	SuccessCount  int64         // 成功次数
	FailCount     int64         // 失败次数
	SkipCount     int64         // 跳过次数因 Singleton 或 Distributed
	LastRunAt     time.Time     // 上次执行时间
	LastSuccessAt time.Time     // 上次成功时间
	LastFailAt    time.Time     // 上次失败时间
	LastError     error         // 上次错误
	LastDuration  time.Duration // 上次执行耗时
	TotalDuration time.Duration // 总执行耗时
}

// Clone 返回统计信息副本.
func (s *JobStats) Clone() JobStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return JobStats{
		RunCount:      s.RunCount,
		SuccessCount:  s.SuccessCount,
		FailCount:     s.FailCount,
		SkipCount:     s.SkipCount,
		LastRunAt:     s.LastRunAt,
		LastSuccessAt: s.LastSuccessAt,
		LastFailAt:    s.LastFailAt,
		LastError:     s.LastError,
		LastDuration:  s.LastDuration,
		TotalDuration: s.TotalDuration,
	}
}

// recordStart 记录任务开始.
func (s *JobStats) recordStart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RunCount++
	s.LastRunAt = time.Now()
}

// recordSuccess 记录任务成功.
func (s *JobStats) recordSuccess(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SuccessCount++
	s.LastSuccessAt = time.Now()
	s.LastDuration = duration
	s.TotalDuration += duration
	s.LastError = nil
}

// recordFail 记录任务失败.
func (s *JobStats) recordFail(duration time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailCount++
	s.LastFailAt = time.Now()
	s.LastDuration = duration
	s.TotalDuration += duration
	s.LastError = err
}

// recordSkip 记录任务跳过.
func (s *JobStats) recordSkip() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SkipCount++
}

// Validate 验证任务配置.
func (j *Job) Validate() error {
	if j.Name == "" {
		return ErrJobNameEmpty
	}
	if j.Schedule == "" {
		return ErrScheduleEmpty
	}
	if j.Handler == nil {
		return ErrHandlerNil
	}
	return nil
}

// State 获取任务状态.
func (j *Job) State() JobState {
	return JobState(j.state.Load())
}

// IsRunning 检查任务是否正在执行.
func (j *Job) IsRunning() bool {
	return j.State() == JobStateRunning
}

// Stats 获取任务统计信息.
func (j *Job) Stats() JobStats {
	j.initStats()
	return j.stats.Clone()
}

// initStats 初始化统计信息.
func (j *Job) initStats() {
	j.statsOnce.Do(func() {
		j.stats = &JobStats{}
	})
}

// tryStart 尝试开始执行CAS 操作，保证单例.
func (j *Job) tryStart() bool {
	return j.state.CompareAndSwap(int32(JobStateIdle), int32(JobStateRunning))
}

// finish 完成执行.
func (j *Job) finish() {
	j.state.Store(int32(JobStateIdle))
}

// JobBuilder 任务构建器.
type JobBuilder struct {
	job *Job
}

// NewJob 创建任务构建器.
func NewJob(name string) *JobBuilder {
	return &JobBuilder{
		job: &Job{
			Name: name,
		},
	}
}

// Schedule 设置调度表达式.
func (b *JobBuilder) Schedule(expr string) *JobBuilder {
	b.job.Schedule = expr
	return b
}

// Handler 设置处理函数.
func (b *JobBuilder) Handler(fn JobFunc) *JobBuilder {
	b.job.Handler = fn
	return b
}

// Timeout 设置超时时间.
func (b *JobBuilder) Timeout(d time.Duration) *JobBuilder {
	b.job.Timeout = d
	return b
}

// Singleton 启用单例模式.
func (b *JobBuilder) Singleton() *JobBuilder {
	b.job.Singleton = true
	return b
}

// Distributed 启用分布式模式.
func (b *JobBuilder) Distributed() *JobBuilder {
	b.job.Distributed = true
	return b
}

// Retry 设置重试策略.
func (b *JobBuilder) Retry(count int, interval time.Duration) *JobBuilder {
	b.job.RetryCount = count
	b.job.RetryInterval = interval
	return b
}

// Build 构建任务.
func (b *JobBuilder) Build() (*Job, error) {
	if err := b.job.Validate(); err != nil {
		return nil, err
	}
	b.job.initStats()
	return b.job, nil
}

// MustBuild 构建任务，失败时 panic.
func (b *JobBuilder) MustBuild() *Job {
	job, err := b.Build()
	if err != nil {
		panic(err)
	}
	return job
}
