# collections/priorityqueue

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/priorityqueue"
```

## 简介

`collections/priorityqueue` 提供基于二叉堆实现的优先队列，支持 O(log n) 的插入和弹出。提供最小堆、最大堆的便捷构造函数，以及支持自定义比较函数的通用构造函数。另有线程安全版本 `ConcurrentPriorityQueue`。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `PriorityQueue[T]` | 优先队列 |
| `New[T](less)` | 自定义优先级（less(a,b)=true 表示 a 优先） |
| `NewMin[T cmp.Ordered]()` | 最小堆 |
| `NewMax[T cmp.Ordered]()` | 最大堆 |
| `Push(items...)` | 添加元素 |
| `Pop()` | 弹出优先级最高的元素 |
| `Peek()` | 查看堆顶（不弹出） |
| `Clone()` / `ToSlice()` | 克隆/转为有序切片（会清空队列） |
| `ConcurrentPriorityQueue[T]` | 线程安全版本 |
| `NewConcurrent[T](less)` / `NewConcurrentMin/Max[T]()` | 创建线程安全优先队列 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/priorityqueue"
)

type Task struct {
    Name     string
    Priority int
}

func main() {
    // 最小堆
    minPQ := priorityqueue.NewMin[int]()
    minPQ.Push(5, 1, 3, 2, 4)
    for minPQ.Len() > 0 {
        v, _ := minPQ.Pop()
        fmt.Print(v, " ") // 1 2 3 4 5
    }
    fmt.Println()

    // 最大堆
    maxPQ := priorityqueue.NewMax[int]()
    maxPQ.Push(5, 1, 3)
    top, _ := maxPQ.Peek()
    fmt.Println("最大值:", top) // 5

    // 自定义优先级（优先级数字越大越优先）
    taskPQ := priorityqueue.New(func(a, b Task) bool {
        return a.Priority > b.Priority
    })
    taskPQ.Push(
        Task{"low", 1},
        Task{"high", 10},
        Task{"medium", 5},
    )
    t, _ := taskPQ.Pop()
    fmt.Println("最高优先级任务:", t.Name) // high

    // 线程安全版本
    cpq := priorityqueue.NewConcurrentMin[int]()
    cpq.Push(3, 1, 2)
    v, _ := cpq.Pop()
    fmt.Println("并发队列弹出:", v) // 1
}
```
