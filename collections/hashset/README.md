# collections/hashset

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/hashset"
```

## 简介

`collections/hashset` 提供基于 `map` 实现的无序集合（HashSet）。Add/Remove/Contains 操作均为 O(1)，支持集合运算（并集、交集、差集、对称差集）以及子集/超集判断。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `HashSet[T comparable]` | 无序集合 |
| `New[T](items...)` | 创建集合 |
| `FromSlice[T](items)` | 从切片创建集合 |
| `Add(items...)` / `Remove(items...)` | 添加/删除元素 |
| `Contains(item)` | 判断元素是否存在 |
| `Union(other)` | 并集 |
| `Intersection(other)` | 交集 |
| `Difference(other)` | 差集（s - other） |
| `SymmetricDifference(other)` | 对称差集 |
| `IsSubset(other)` / `IsSuperset(other)` | 子集/超集判断 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/hashset"
)

func main() {
    s1 := hashset.New(1, 2, 3, 4)
    s2 := hashset.New(3, 4, 5, 6)

    fmt.Println("s1 包含 3:", s1.Contains(3))  // true
    fmt.Println("s1 大小:", s1.Len())           // 4

    // 集合运算
    union := s1.Union(s2)
    inter := s1.Intersection(s2)
    diff  := s1.Difference(s2)
    symDiff := s1.SymmetricDifference(s2)

    fmt.Println("并集大小:", union.Len())       // 6
    fmt.Println("交集:", inter.ToSlice())       // [3 4] (顺序不定)
    fmt.Println("差集:", diff.ToSlice())        // [1 2] (顺序不定)
    fmt.Println("对称差集大小:", symDiff.Len()) // 4

    // 子集判断
    small := hashset.New(3, 4)
    fmt.Println("small 是 s1 的子集:", small.IsSubset(s1)) // true

    // 遍历（顺序不确定）
    s1.Range(func(item int) bool {
        fmt.Print(item, " ")
        return true // 返回 false 停止
    })
    fmt.Println()
}
```
