# Scheduler

任务调度器模块，提供基于 Cron 表达式的定时任务调度功能。

## 特性

- **秒级 Cron 表达式** - 支持 6 字段格式
- **单例模式** - 防止同一任务重叠执行（本地幂等）
- **分布式锁** - 多实例部署时保证只有一个实例执行（分布式幂等）
- **任务状态跟踪** - 实时查看任务状态和执行统计
- **Hook 机制** - BeforeJob/AfterJob/OnError/OnSkip 回调
- **失败重试** - 可配置重试次数和间隔
- **优雅关闭** - 等待正在执行的任务完成
- **Builder 模式** - 链式 API 创建任务

## 安装

```go
import "github.com/Tsukikage7/servex/v2/scheduler"
```

## 幂等性保证

### 单例模式（本地幂等）

防止同一任务在单实例内重叠执行：

```go
scheduler.NewJob("task").
    Schedule("*/10 * * * * *").  // 每 10 秒
    Handler(longRunningHandler). // 执行时间可能超过 10 秒
    Singleton().                 // 上一次未完成时跳过本次
    MustBuild()
```

### 分布式锁（分布式幂等）

多实例部署时保证只有一个实例执行，直接复用 `cache.Cache` 接口：

```go
// 使用 Redis 缓存的分布式锁
redisCache, _ := cache.New(&cache.Config{
    Type:    "redis",
    Address: "localhost:6379",
})

s := scheduler.MustNew(
    scheduler.WithCache(redisCache),              // 复用 cache 包
    scheduler.WithLockPrefix("myapp:scheduler:"), // 可选，自定义前缀
    scheduler.WithLockTTL(15*time.Minute),
)

s.Add(scheduler.NewJob("report").
    Schedule("0 0 * * * *").
    Handler(generateReportHandler).
    Distributed().  // 启用分布式锁
    Singleton().    // 同时启用本地单例
    MustBuild(),
)
```

## Hook 机制

```go
hooks := scheduler.NewHooks().
    BeforeJob(func(ctx context.Context, jc *scheduler.JobContext) error {
        log.Infof("任务开始: %s", jc.Job.Name)
        return nil  // 返回 error 将阻止任务执行
    }).
    AfterJob(func(ctx context.Context, jc *scheduler.JobContext) {
        log.Infof("任务完成: %s [duration:%v]", jc.Job.Name, jc.Duration)
    }).
    OnError(func(ctx context.Context, jc *scheduler.JobContext) {
        log.Errorf("任务失败: %s [error:%v]", jc.Job.Name, jc.Error)
        // 发送告警通知
    }).
    OnSkip(func(ctx context.Context, jc *scheduler.JobContext) {
        log.Warnf("任务跳过: %s [reason:%s]", jc.Job.Name, jc.SkipReason)
    }).
    Build()

s := scheduler.MustNew(
    scheduler.WithHooks(hooks),
)
```

## 任务统计

```go
job, _ := s.Get("sync-data")
stats := job.Stats()

fmt.Printf("执行次数: %d\n", stats.RunCount)
fmt.Printf("成功次数: %d\n", stats.SuccessCount)
fmt.Printf("失败次数: %d\n", stats.FailCount)
fmt.Printf("跳过次数: %d\n", stats.SkipCount)
fmt.Printf("上次执行: %v\n", stats.LastRunAt)
fmt.Printf("上次耗时: %v\n", stats.LastDuration)
fmt.Printf("总执行时间: %v\n", stats.TotalDuration)

if stats.LastError != nil {
    fmt.Printf("上次错误: %v\n", stats.LastError)
}
```

## API 参考

### 调度器选项

| 选项                     | 说明                       | 默认值            |
| ------------------------ | -------------------------- | ----------------- |
| `WithLogger(log)`        | 日志记录器                 | nil               |
| `WithCache(cache)`       | 缓存客户端（用于分布式锁） | nil               |
| `WithLockPrefix(prefix)` | 分布式锁 key 前缀          | "scheduler:lock:" |
| `WithInstanceID(id)`     | 实例 ID                    | 自动生成          |
| `WithHooks(hooks)`       | 全局钩子                   | nil               |
| `WithDefaultTimeout(d)`  | 默认任务超时               | 5 分钟            |
| `WithLockTTL(d)`         | 分布式锁过期时间           | 10 分钟           |
| `WithSeconds(bool)`      | 秒级调度                   | true              |
| `WithLocation(loc)`      | 时区                       | time.Local        |

### 任务 Builder

```go
scheduler.NewJob("name").
    Schedule("cron expression").  // 必填
    Handler(func(ctx) error).     // 必填
    Timeout(5*time.Minute).       // 可选
    Singleton().                  // 可选：本地单例
    Distributed().                // 可选：分布式锁
    Retry(3, 10*time.Second).     // 可选：重试
    Build()                       // 或 MustBuild()
```

### 调度器方法

```go
s.Add(job)           // 添加任务
s.Remove("name")     // 移除任务
s.Get("name")        // 获取任务
s.List()             // 列出所有任务
s.Start()            // 启动
s.Stop()             // 停止
s.Shutdown(ctx)      // 优雅关闭
s.Running()          // 是否运行中
s.Trigger("name")    // 立即触发任务
```

## Cron 表达式

支持秒级 6 字段格式：

```
┌───────────── 秒 (0-59)
│ ┌───────────── 分 (0-59)
│ │ ┌───────────── 时 (0-23)
│ │ │ ┌───────────── 日 (1-31)
│ │ │ │ ┌───────────── 月 (1-12)
│ │ │ │ │ ┌───────────── 周 (0-6, 0=周日)
│ │ │ │ │ │
* * * * * *
```

### 常用表达式

| 表达式          | 说明        |
| --------------- | ----------- |
| `0 0 * * * *`   | 每小时整点  |
| `0 */5 * * * *` | 每 5 分钟   |
| `0 0 0 * * *`   | 每天午夜    |
| `0 0 9 * * 1-5` | 工作日 9 点 |
| `@every 30s`    | 每 30 秒    |
| `@hourly`       | 每小时      |
| `@daily`        | 每天午夜    |

## 错误处理

```go
import "errors"

err := s.Add(job)

switch {
case errors.Is(err, scheduler.ErrJobExists):
    // 任务已存在
case errors.Is(err, scheduler.ErrScheduleInvalid):
    // Cron 表达式无效
case errors.Is(err, scheduler.ErrSchedulerClosed):
    // 调度器已关闭
}
```

## 最佳实践

1. **始终启用 Singleton** - 除非任务执行时间确保小于调度间隔
2. **分布式部署启用 Distributed** - 避免多实例重复执行
3. **设置合理的超时时间** - 防止任务无限执行
4. **使用 Shutdown 而非 Stop** - 等待任务完成
5. **配置 OnError Hook** - 及时发现任务失败
6. **LockTTL 大于任务超时** - 防止锁提前释放
