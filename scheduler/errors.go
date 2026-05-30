package scheduler

import "errors"

var (
	// ErrJobNameEmpty 任务名称为空.
	ErrJobNameEmpty = errors.New("scheduler: 任务名称不能为空")

	// ErrScheduleEmpty 调度表达式为空.
	ErrScheduleEmpty = errors.New("scheduler: 调度表达式不能为空")

	// ErrHandlerNil 任务处理函数为空.
	ErrHandlerNil = errors.New("scheduler: 任务处理函数不能为空")

	// ErrScheduleInvalid 无效的调度表达式.
	ErrScheduleInvalid = errors.New("scheduler: 无效的调度表达式")

	// ErrSchedulerClosed 调度器已关闭.
	ErrSchedulerClosed = errors.New("scheduler: 调度器已关闭")

	// ErrJobNotFound 任务未找到.
	ErrJobNotFound = errors.New("scheduler: 任务未找到")

	// ErrJobExists 任务已存在.
	ErrJobExists = errors.New("scheduler: 任务已存在")

	// ErrJobRunning 任务正在执行中.
	ErrJobRunning = errors.New("scheduler: 任务正在执行中")

	// ErrLockAcquireFailed 获取锁失败.
	ErrLockAcquireFailed = errors.New("scheduler: 获取锁失败")

	// ErrJobSkipped 任务被跳过上一次执行未完成.
	ErrJobSkipped = errors.New("scheduler: 任务被跳过，上一次执行未完成")
)
