# collections/treeset

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/treeset"
```

## 简介

`collections/treeset` 提供基于红黑树实现的有序集合，元素按排序顺序存储，Add/Remove/Contains 操作时间复杂度 O(log n)，不允许重复元素。支持集合运算（并集、交集、差集）。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `TreeSet[T]` | 有序集合 |
| `New[T](cmp)` | 自定义比较器创建 |
| `NewOrdered[T cmp.Ordered]()` | 内置有序类型快速创建 |
| `FromSlice[T cmp.Ordered](items)` | 从切片创建 |
| `Add(items...)` / `Remove(items...)` | 添加/删除元素 |
| `Contains(item)` | 是否包含 |
| `First()` / `Last()` | 最小/最大元素 |
| `ToSlice()` | 按序返回所有元素 |
| `Union/Intersection/Difference` | 集合运算 |
| `IsSubset/IsSuperset/Equal` | 关系判断 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/treeset"
)

func main() {
    s1 := treeset.NewOrdered[int]()
    s1.Add(5, 1, 3, 2, 4)

    // 有序输出
    fmt.Println("有序元素:", s1.ToSlice()) // [1 2 3 4 5]

    // 极值
    first, _ := s1.First()
    last, _ := s1.Last()
    fmt.Println("最小:", first, "最大:", last) // 1  5

    // 集合运算
    s2 := treeset.FromSlice([]int{3, 4, 5, 6, 7})
    inter := s1.Intersection(s2)
    union := s1.Union(s2)
    diff  := s1.Difference(s2)

    fmt.Println("交集:", inter.ToSlice()) // [3 4 5]
    fmt.Println("并集:", union.ToSlice()) // [1 2 3 4 5 6 7]
    fmt.Println("差集:", diff.ToSlice())  // [1 2]

    // 有序遍历（支持提前停止）
    s1.Range(func(item int) bool {
        fmt.Print(item, " ")
        return item < 4
    })
    fmt.Println()

    // 字符串有序集合
    words := treeset.NewOrdered[string]()
    words.Add("banana", "apple", "cherry")
    fmt.Println("有序单词:", words.ToSlice()) // [apple banana cherry]
}
```
