# collections/blockingqueue

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/blockingqueue"
```

## 简介

`collections/blockingqueue` 提供基于环形缓冲区的有界阻塞队列。使用双信号量实现阻塞语义：队列满时 `Enqueue` 阻塞，队列空时 `Dequeue` 阻塞，支持 `context.Context` 取消。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `BlockingQueue[T]` | 阻塞队列接口，定义 Enqueue/Dequeue/Len/IsFull/IsEmpty |
| `ArrayBlockingQueue[T]` | 基于环形缓冲区的有界阻塞队列实现 |
| `New[T](capacity)` | 创建指定容量的阻塞队列，capacity <= 0 会 panic |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/Tsukikage7/servex/v2/collections/blockingqueue"
)

func main() {
    // 创建容量为 3 的阻塞队列
    q := blockingqueue.New[int](3)

    ctx := context.Background()

    var wg sync.WaitGroup

    // 生产者：入队
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 1; i <= 5; i++ {
            if err := q.Enqueue(ctx, i); err != nil {
                fmt.Println("入队失败:", err)
                return
            }
            fmt.Println("入队:", i)
        }
    }()

    // 消费者：出队
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 5; i++ {
            val, err := q.Dequeue(ctx)
            if err != nil {
                fmt.Println("出队失败:", err)
                return
            }
            fmt.Println("出队:", val)
        }
    }()

    wg.Wait()

    // 支持超时取消
    ctxTimeout, cancel := context.WithTimeout(ctx, 0)
    defer cancel()
    _, err := q.Dequeue(ctxTimeout) // 立即返回 context.DeadlineExceeded
    fmt.Println("超时出队:", err)
}
```
