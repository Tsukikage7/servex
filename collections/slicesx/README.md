# collections/slicesx

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/slicesx"
```

## 简介

`collections/slicesx` 提供丰富的切片操作泛型工具函数，涵盖函数式编程常用操作（Filter/Map/Reduce）、集合运算（交集/并集/差集）、分组、分块、去重、数值统计等。

## 核心函数

| 函数 | 说明 |
|---|---|
| `Filter(slice, fn)` | 过滤满足条件的元素 |
| `Map(slice, fn)` | 元素映射转换 |
| `Reduce(slice, init, fn)` | 归约 |
| `Unique(slice)` / `UniqueBy(slice, keyFn)` | 去重 |
| `GroupBy(slice, keyFn)` | 分组 |
| `Chunk(slice, size)` | 分块 |
| `Partition(slice, fn)` | 分区（满足/不满足） |
| `Find(slice, fn)` / `FindIndex(slice, fn)` | 查找 |
| `Any/All/None/Count` | 断言统计 |
| `Flatten(slices)` | 展平二维切片 |
| `Zip(keys, values)` | 拉链合并 |
| `IntersectSet/UnionSet/DiffSet` | 集合运算 |
| `Sum/Min/Max` | 数值统计 |
| `Insert/Delete` | 安全插入/删除 |
| `Reverse/ReverseSelf` | 反转 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/slicesx"
)

func main() {
    nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

    // 过滤偶数
    evens := slicesx.Filter(nums, func(n int) bool { return n%2 == 0 })
    fmt.Println("偶数:", evens) // [2 4 6 8 10]

    // 映射转换
    doubled := slicesx.Map(evens, func(n int) int { return n * 2 })
    fmt.Println("翻倍:", doubled) // [4 8 12 16 20]

    // 求和
    fmt.Println("和:", slicesx.Sum(nums)) // 55

    // 去重
    dup := []int{1, 2, 2, 3, 1, 4}
    fmt.Println("去重:", slicesx.Unique(dup)) // [1 2 3 4]

    // 分组
    groups := slicesx.GroupBy(nums, func(n int) string {
        if n%2 == 0 { return "偶" }
        return "奇"
    })
    fmt.Println("奇数组:", groups["奇"])

    // 分块
    chunks := slicesx.Chunk(nums, 3)
    fmt.Println("分块:", chunks) // [[1 2 3] [4 5 6] [7 8 9] [10]]

    // 集合运算
    a := []int{1, 2, 3, 4}
    b := []int{3, 4, 5, 6}
    fmt.Println("交集:", slicesx.IntersectSet(a, b)) // [3 4]
    fmt.Println("并集:", slicesx.UnionSet(a, b))     // [1 2 3 4 5 6]
    fmt.Println("差集:", slicesx.DiffSet(a, b))      // [1 2]
}
```
