# collections/mapsx

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/mapsx"
```

## 简介

`collections/mapsx` 提供 `map` 操作的泛型工具函数集合，包含键/值提取、过滤、转换、合并、差异比较等常用操作。所有函数均基于 Go 泛型，无额外依赖。

## 核心函数

| 函数 | 说明 |
|---|---|
| `Keys(m)` / `Values(m)` | 返回所有键/值切片 |
| `Entries(m)` / `FromEntries(entries)` | 键值对互转 |
| `Merge(maps...)` | 合并多个 map（后者覆盖前者） |
| `MergeFunc(fn, maps...)` | 自定义冲突合并函数 |
| `Filter(m, fn)` | 过滤满足条件的键值对 |
| `FilterKeys(m, keys...)` / `OmitKeys(m, keys...)` | 按键白名单/黑名单过滤 |
| `MapKeys(m, fn)` / `MapValues(m, fn)` | 转换键/值 |
| `Invert(m)` | 键值互换 |
| `Clone(m)` | 浅拷贝 |
| `Equal(m1, m2)` | 判断相等 |
| `GetOrDefault(m, key, def)` / `GetOrPut(m, key, def)` | 默认值获取 |
| `Diff(m1, m2)` | 返回 added/removed/changed 键列表 |

## 示例

```go
package main

import (
    "fmt"
    "strconv"

    "github.com/Tsukikage7/servex/v2/collections/mapsx"
)

func main() {
    m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

    // 提取键/值
    keys := mapsx.Keys(m)
    fmt.Println("键数量:", len(keys)) // 4

    // 过滤值 > 2 的条目
    filtered := mapsx.Filter(m, func(k string, v int) bool {
        return v > 2
    })
    fmt.Println("过滤后:", filtered) // map[c:3 d:4]

    // 转换值
    doubled := mapsx.MapValues(m, func(v int) int { return v * 2 })
    fmt.Println("翻倍:", doubled)

    // 转换键（int -> string）
    numMap := map[int]string{1: "a", 2: "b"}
    strMap := mapsx.MapKeys(numMap, strconv.Itoa)
    fmt.Println("键转换:", strMap) // map[1:a 2:b]

    // 合并（后者覆盖前者）
    m1 := map[string]int{"x": 1}
    m2 := map[string]int{"x": 10, "y": 2}
    merged := mapsx.Merge(m1, m2)
    fmt.Println("合并:", merged) // map[x:10 y:2]

    // 差异比较
    added, removed, changed := mapsx.Diff(m1, m2)
    fmt.Println("added:", added, "removed:", removed, "changed:", changed)
}
```
