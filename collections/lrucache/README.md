# collections/lrucache

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/collections/lrucache"
```

## 简介

`collections/lrucache` 提供线程安全的 LRU（Least Recently Used）缓存实现，基于哈希表 + 双向链表，Get/Put 操作时间复杂度 O(1)。缓存满时自动淘汰最近最少使用的条目。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `LRUCache[K, V]` | LRU 缓存 |
| `New[K, V](capacity)` | 创建缓存，capacity <= 0 自动设为 1 |
| `Get(key)` | 获取并移动到最近使用位置 |
| `Put(key, value)` | 设置缓存，满时淘汰最久未使用条目 |
| `GetOrPut(key, loader)` | 获取，不存在则调用 loader 加载并缓存 |
| `Peek(key)` | 查看值（不影响 LRU 顺序） |
| `Remove(key)` | 删除缓存项 |
| `Resize(capacity)` | 动态调整容量 |
| `Keys()` | 按最近使用顺序返回所有键 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/collections/lrucache"
)

func main() {
    cache := lrucache.New[string, int](3)

    cache.Put("a", 1)
    cache.Put("b", 2)
    cache.Put("c", 3)

    // 访问 "a"，移到最近位置
    val, ok := cache.Get("a")
    fmt.Println("a:", val, ok) // 1 true

    // 插入第四个，淘汰最久未使用的 "b"
    cache.Put("d", 4)

    _, ok = cache.Get("b")
    fmt.Println("b 已被淘汰:", !ok) // true

    // GetOrPut：不存在则加载
    v := cache.GetOrPut("e", func() int { return 5 })
    fmt.Println("e:", v) // 5

    // Peek 不影响顺序
    v2, _ := cache.Peek("a")
    fmt.Println("peek a:", v2)

    fmt.Println("大小:", cache.Len())         // 3
    fmt.Println("容量:", cache.Capacity())    // 3
    fmt.Println("键（按使用顺序）:", cache.Keys())
}
```
