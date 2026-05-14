# collections/linkedmap

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/linkedmap"
```

## 简介

`collections/linkedmap` 提供维护插入顺序的 Map 实现，基于哈希表 + 双向链表。`Put/Get/Remove` 操作均为 O(1)，`Keys()/Values()/Range()` 按插入顺序返回。更新已存在键的值时不改变其顺序。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `LinkedMap[K, V]` | 维护插入顺序的 Map |
| `New[K, V]()` | 创建空 LinkedMap |
| `Put(key, value)` | 插入或更新（不改变已有键的顺序） |
| `Get(key)` | 获取值 |
| `Remove(key)` | 删除键值对 |
| `Keys()` | 按插入顺序返回所有键 |
| `Values()` | 按插入顺序返回所有值 |
| `Range(fn)` | 按插入顺序遍历，fn 返回 false 停止 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/linkedmap"
)

func main() {
    m := linkedmap.New[string, int]()

    // 按顺序插入
    m.Put("banana", 2)
    m.Put("apple", 1)
    m.Put("cherry", 3)

    // Keys 保持插入顺序
    fmt.Println("键顺序:", m.Keys())   // [banana apple cherry]
    fmt.Println("值顺序:", m.Values()) // [2 1 3]

    // 更新不改变顺序
    m.Put("banana", 20)
    fmt.Println("更新后键顺序:", m.Keys()) // [banana apple cherry]

    // 获取值
    val, ok := m.Get("apple")
    fmt.Println("apple:", val, ok) // 1 true

    // 删除
    m.Remove("apple")
    fmt.Println("删除后:", m.Keys()) // [banana cherry]

    // 遍历
    m.Range(func(k string, v int) bool {
        fmt.Printf("%s=%d ", k, v)
        return true
    })
    fmt.Println()

    fmt.Println("大小:", m.Len()) // 2
}
```
