# collections/delayqueue

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/delayqueue"
```

## 简介

`collections/delayqueue` 提供基于优先队列的延迟队列。元素须实现 `Delayable` 接口，只有 `Delay() <= 0`（即到期）的元素才能被出队。`Dequeue` 会阻塞直到有元素到期或 `context` 取消。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Delayable` | 接口，`Delay() time.Duration` 返回距到期的剩余时间 |
| `DelayQueue[T Delayable]` | 延迟队列 |
| `New[T](capacity)` | 创建延迟队列 |
| `Enqueue(ctx, item)` | 入队，新元素若成为堆顶会唤醒等待的 Dequeue |
| `Dequeue(ctx)` | 出队，阻塞直到有元素到期 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/v2/collections/delayqueue"
)

// Task 实现 Delayable 接口
type Task struct {
    ID       string
    ExpireAt time.Time
}

func (t *Task) Delay() time.Duration {
    return time.Until(t.ExpireAt)
}

func main() {
    dq := delayqueue.New[*Task](16)
    ctx := context.Background()

    now := time.Now()

    // 入队三个任务，延迟不同
    dq.Enqueue(ctx, &Task{ID: "task-3", ExpireAt: now.Add(300 * time.Millisecond)})
    dq.Enqueue(ctx, &Task{ID: "task-1", ExpireAt: now.Add(100 * time.Millisecond)})
    dq.Enqueue(ctx, &Task{ID: "task-2", ExpireAt: now.Add(200 * time.Millisecond)})

    // 按到期顺序出队
    for i := 0; i < 3; i++ {
        task, err := dq.Dequeue(ctx)
        if err != nil {
            fmt.Println("出队失败:", err)
            return
        }
        fmt.Printf("处理任务: %s，实际延迟: %v\n", task.ID, time.Since(now))
    }
    // 输出顺序: task-1, task-2, task-3
}
```
