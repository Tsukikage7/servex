package scheduler

import (
	"context"
	"time"
)

// JobContext 任务执行上下文.
type JobContext struct {
	// Job 当前任务.
	Job *Job

	// StartTime 开始执行时间.
	StartTime time.Time

	// Attempt 当前重试次数从 1 开始.
	Attempt int

	// Error 执行错误仅在 AfterJob/OnError 中有值.
	Error error

	// Duration 执行耗时仅在 AfterJob/OnError 中有值.
	Duration time.Duration

	// Skipped 是否被跳过.
	Skipped bool

	// SkipReason 跳过原因.
	SkipReason string
}

// BeforeJobHook 任务执行前回调.
// 返回 error 将阻止任务执行.
type BeforeJobHook func(ctx context.Context, jc *JobContext) error

// AfterJobHook 任务执行后回调.
type AfterJobHook func(ctx context.Context, jc *JobContext)

// OnErrorHook 任务错误回调.
type OnErrorHook func(ctx context.Context, jc *JobContext)

// OnSkipHook 任务跳过回调.
type OnSkipHook func(ctx context.Context, jc *JobContext)

// Hooks 任务钩子集合.
type Hooks struct {
	// BeforeJob 任务执行前回调列表.
	BeforeJob []BeforeJobHook

	// AfterJob 任务执行后回调列表无论成功失败都会调用.
	AfterJob []AfterJobHook

	// OnError 任务错误回调列表.
	OnError []OnErrorHook

	// OnSkip 任务跳过回调列表.
	OnSkip []OnSkipHook
}

// runBeforeHooks 执行前置钩子.
func (h *Hooks) runBeforeHooks(ctx context.Context, jc *JobContext) error {
	if h == nil {
		return nil
	}
	for _, hook := range h.BeforeJob {
		if err := hook(ctx, jc); err != nil {
			return err
		}
	}
	return nil
}

// runAfterHooks 执行后置钩子.
func (h *Hooks) runAfterHooks(ctx context.Context, jc *JobContext) {
	if h == nil {
		return
	}
	for _, hook := range h.AfterJob {
		hook(ctx, jc)
	}
}

// runErrorHooks 执行错误钩子.
func (h *Hooks) runErrorHooks(ctx context.Context, jc *JobContext) {
	if h == nil {
		return
	}
	for _, hook := range h.OnError {
		hook(ctx, jc)
	}
}

// runSkipHooks 执行跳过钩子.
func (h *Hooks) runSkipHooks(ctx context.Context, jc *JobContext) {
	if h == nil {
		return
	}
	for _, hook := range h.OnSkip {
		hook(ctx, jc)
	}
}

// HooksBuilder 钩子构建器.
type HooksBuilder struct {
	hooks *Hooks
}

// NewHooks 创建钩子构建器.
func NewHooks() *HooksBuilder {
	return &HooksBuilder{
		hooks: &Hooks{},
	}
}

// BeforeJob 添加前置钩子.
func (b *HooksBuilder) BeforeJob(hook BeforeJobHook) *HooksBuilder {
	b.hooks.BeforeJob = append(b.hooks.BeforeJob, hook)
	return b
}

// AfterJob 添加后置钩子.
func (b *HooksBuilder) AfterJob(hook AfterJobHook) *HooksBuilder {
	b.hooks.AfterJob = append(b.hooks.AfterJob, hook)
	return b
}

// OnError 添加错误钩子.
func (b *HooksBuilder) OnError(hook OnErrorHook) *HooksBuilder {
	b.hooks.OnError = append(b.hooks.OnError, hook)
	return b
}

// OnSkip 添加跳过钩子.
func (b *HooksBuilder) OnSkip(hook OnSkipHook) *HooksBuilder {
	b.hooks.OnSkip = append(b.hooks.OnSkip, hook)
	return b
}

// Build 构建钩子.
func (b *HooksBuilder) Build() *Hooks {
	return b.hooks
}
