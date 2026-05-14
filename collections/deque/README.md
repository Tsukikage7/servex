# collections/deque

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/deque"
```

## 简介

`collections/deque` 提供双端队列（Double-Ended Queue）实现，基于环形缓冲区，支持在头部和尾部进行 O(1) 插入和删除。自动扩容（满时容量翻倍）和缩容（元素数量降至容量四分之一时缩半）。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Deque[T]` | 双端队列 |
| `New[T]()` | 创建空双端队列（初始容量 8） |
| `NewWithCapacity[T](n)` | 创建指定初始容量的双端队列 |
| `From[T](items)` | 从切片创建双端队列 |
| `PushFront(item)` / `PushBack(item)` | 头部/尾部添加元素 |
| `PopFront()` / `PopBack()` | 头部/尾部移除并返回元素 |
| `PeekFront()` / `PeekBack()` | 查看头部/尾部元素（不移除） |
| `At(index)` / `Set(index, item)` | 随机访问 |
| `Rotate(n)` / `Reverse()` | 旋转/反转 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/deque"
)

func main() {
    dq := deque.New[int]()

    // 两端插入
    dq.PushBack(2)
    dq.PushBack(3)
    dq.PushFront(1)
    dq.PushFront(0)
    // 队列: [0, 1, 2, 3]

    fmt.Println("长度:", dq.Len()) // 4

    // 查看两端
    front, _ := dq.PeekFront()
    back, _ := dq.PeekBack()
    fmt.Println("头部:", front, "尾部:", back) // 0  3

    // 随机访问
    val, _ := dq.At(2)
    fmt.Println("索引2:", val) // 2

    // 两端弹出
    v1, _ := dq.PopFront()
    v2, _ := dq.PopBack()
    fmt.Println("弹出头:", v1, "弹出尾:", v2) // 0  3

    // 转为切片
    fmt.Println("切片:", dq.ToSlice()) // [1, 2]

    // 从切片构建并反转
    dq2 := deque.From([]string{"a", "b", "c", "d"})
    dq2.Reverse()
    fmt.Println("反转:", dq2.ToSlice()) // [d, c, b, a]
}
```
