# collections/multimap

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/multimap"
```

## 简介

`collections/multimap` 提供一对多映射（MultiMap）实现，一个键可对应多个值。基于 `map[K][]V` 实现，维护每个键下的有序值切片。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `MultiMap[K, V]` | 一对多映射 |
| `New[K, V]()` | 创建空 MultiMap |
| `Put(key, value)` | 向 key 追加一个值 |
| `PutAll(key, values...)` | 向 key 追加多个值 |
| `Get(key)` | 返回 key 对应的所有值切片 |
| `Remove(key)` | 移除整个 key 及其所有值 |
| `RemoveValue[K,V](m, key, value)` | 移除 key 下特定值（第一次出现） |
| `ContainsKey(key)` | 判断是否包含指定键 |
| `Len()` | 总键值对数 |
| `KeyLen()` | 键的数量 |
| `Range(fn)` | 遍历，fn 接收 (key, []value) |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/multimap"
)

func main() {
    m := multimap.New[string, int]()

    // 一个键对应多个值
    m.Put("scores", 90)
    m.Put("scores", 85)
    m.Put("scores", 92)
    m.PutAll("tags", 1, 2, 3)

    fmt.Println("scores:", m.Get("scores")) // [90 85 92]
    fmt.Println("tags:", m.Get("tags"))     // [1 2 3]

    fmt.Println("总键值对数:", m.Len())    // 6
    fmt.Println("键数量:", m.KeyLen())     // 2

    // 移除特定值
    multimap.RemoveValue(m, "scores", 85)
    fmt.Println("移除85后:", m.Get("scores")) // [90 92]

    // 遍历
    m.Range(func(key string, vals []int) bool {
        fmt.Printf("%s -> %v\n", key, vals)
        return true
    })

    // 移除整个键
    m.Remove("tags")
    fmt.Println("移除tags后键数量:", m.KeyLen()) // 1

    // 不存在的键返回 nil
    fmt.Println("不存在的键:", m.Get("unknown")) // []
}
```
