# collections/treemap

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/treemap"
```

## 简介

`collections/treemap` 提供基于红黑树实现的有序 Map，键按自定义比较器排序存储。Put/Get/Remove 操作时间复杂度 O(log n)，遍历时按键有序。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `TreeMap[K, V]` | 有序 Map |
| `New[K, V](cmp)` | 自定义比较器创建 |
| `NewOrdered[K cmp.Ordered, V]()` | 内置有序类型快速创建 |
| `Put/Get/Remove/ContainsKey` | 基本操作 |
| `Keys()/Values()/Entries()` | 按序返回 |
| `FirstKey()/LastKey()/First()/Last()` | 极值操作 |
| `Range(fn)` | 按序遍历，fn 返回 false 停止 |
| `OrderedCompare[T]` | 内置有序类型比较器 |
| `ReverseCompare[T]` | 逆序比较器 |
| `TimeCompare` | 时间比较器 |
| `Reverse(cmp)` | 返回逆序比较器 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/treemap"
)

func main() {
    // 整数键有序 Map
    tm := treemap.NewOrdered[int, string]()
    tm.Put(3, "three")
    tm.Put(1, "one")
    tm.Put(4, "four")
    tm.Put(2, "two")

    // 按键有序输出
    fmt.Println("键:", tm.Keys())   // [1 2 3 4]
    fmt.Println("值:", tm.Values()) // [one two three four]

    // 极值
    first, _ := tm.First()
    last, _ := tm.Last()
    fmt.Printf("最小: %d=%s, 最大: %d=%s\n", first.Key, first.Value, last.Key, last.Value)

    // 范围遍历
    tm.Range(func(k int, v string) bool {
        fmt.Printf("%d:%s ", k, v)
        return k < 3 // 遍历到 3 停止
    })
    fmt.Println()

    // 逆序 Map
    revTM := treemap.New[string, int](treemap.ReverseCompare[string])
    revTM.Put("banana", 2)
    revTM.Put("apple", 1)
    revTM.Put("cherry", 3)
    fmt.Println("逆序键:", revTM.Keys()) // [cherry banana apple]
}
```
