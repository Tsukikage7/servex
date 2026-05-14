# Collections

Go 语言泛型集合工具包，提供常用数据结构和集合操作函数。

## 安装

```go
import "github.com/Tsukikage7/servex/v2/collections"
```

## 数据结构

### TreeMap - 有序映射

基于红黑树实现的有序映射，按键排序，所有操作时间复杂度 O(log n)。

```go
import "github.com/Tsukikage7/servex/v2/collections/treemap"

// 使用内置比较器（适用于 cmp.Ordered 类型）
m := treemap.NewOrdered[string, int]()
m.Put("b", 2)
m.Put("a", 1)
m.Put("c", 3)

// 有序遍历
m.Keys()   // ["a", "b", "c"]
m.Values() // [1, 2, 3]

// 范围查询
m.Range("a", "c") // [("a", 1), ("b", 2)]

// 自定义比较器
m2 := treemap.New[time.Time, string](treemap.TimeCompare)

// 反向排序
m3 := treemap.New[int, string](treemap.ReverseCompare[int])
```

### TreeSet - 有序集合

基于 TreeMap 实现的有序集合。

```go
import "github.com/Tsukikage7/servex/v2/collections/treeset"

s := treeset.NewOrdered[int]()
s.Add(3, 1, 2)
s.Contains(2) // true
s.ToSlice()   // [1, 2, 3]

// 集合运算
s2 := treeset.NewOrdered[int]()
s2.Add(2, 3, 4)

s.Union(s2)        // {1, 2, 3, 4}
s.Intersection(s2) // {2, 3}
s.Difference(s2)   // {1}
```

### HashSet - 哈希集合

基于 map 实现的无序集合，O(1) 操作。

```go
import "github.com/Tsukikage7/servex/v2/collections/hashset"

s := hashset.New[string]("a", "b", "c")
s.Add("d")
s.Contains("a") // true
s.Remove("b")

// 集合运算
s2 := hashset.New[string]("c", "d", "e")
s.Union(s2)              // {"a", "c", "d", "e"}
s.SymmetricDifference(s2) // {"a", "e"}
```

### PriorityQueue - 优先队列

基于二叉堆实现的优先队列。

```go
import "github.com/Tsukikage7/servex/v2/collections/priorityqueue"

// 最小堆
minPQ := priorityqueue.NewMin[int]()
minPQ.Push(3, 1, 4, 1, 5)
minPQ.Pop() // 1
minPQ.Pop() // 1
minPQ.Pop() // 3

// 最大堆
maxPQ := priorityqueue.NewMax[int]()
maxPQ.Push(3, 1, 4, 1, 5)
maxPQ.Pop() // 5
maxPQ.Pop() // 4

// 自定义比较
type Task struct {
    Name     string
    Priority int
}
pq := priorityqueue.New(func(a, b Task) bool {
    return a.Priority > b.Priority // 高优先级先出
})
```

### LRUCache - LRU 缓存

基于哈希表 + 双向链表实现的 LRU 缓存，线程安全。

```go
import "github.com/Tsukikage7/servex/v2/collections/lrucache"

cache := lrucache.New[string, int](100) // 容量 100

cache.Put("a", 1)
cache.Put("b", 2)

val, ok := cache.Get("a") // 1, true（并更新为最近使用）
val, ok = cache.Peek("b") // 2, true（不更新使用时间）

// 带加载函数
val = cache.GetOrPut("c", func() int {
    return loadFromDB("c") // 仅在缓存未命中时调用
})

// 调整容量
cache.Resize(50) // 缩容会淘汰多余元素
```

### Deque - 双端队列

基于环形缓冲区实现的双端队列，支持两端 O(1) 操作。

```go
import "github.com/Tsukikage7/servex/v2/collections/deque"

dq := deque.New[int]()
dq.PushBack(1)
dq.PushBack(2)
dq.PushFront(0)
// [0, 1, 2]

dq.PopFront() // 0
dq.PopBack()  // 2
dq.PeekFront() // 1

// 用作栈（LIFO）
stack := deque.New[string]()
stack.PushBack("a")
stack.PushBack("b")
stack.PopBack() // "b"

// 用作队列（FIFO）
queue := deque.New[string]()
queue.PushBack("a")
queue.PushBack("b")
queue.PopFront() // "a"

// 旋转和反转
dq.Rotate(2)  // 向右旋转
dq.Reverse()  // 反转
```

## 工具函数

### slicesx - 切片操作

函数式切片操作工具。

```go
import "github.com/Tsukikage7/servex/v2/collections/slicesx"

// 过滤
nums := []int{1, 2, 3, 4, 5}
evens := slicesx.Filter(nums, func(n int) bool { return n%2 == 0 })
// [2, 4]

// 映射
strs := slicesx.Map(nums, strconv.Itoa)
// ["1", "2", "3", "4", "5"]

// 归约
sum := slicesx.Reduce(nums, 0, func(acc, n int) int { return acc + n })
// 15

// 去重
slicesx.Unique([]int{1, 2, 2, 3, 1})  // [1, 2, 3]
slicesx.UniqueBy(users, func(u User) int { return u.ID })

// 分组
groups := slicesx.GroupBy(nums, func(n int) string {
    if n%2 == 0 { return "even" }
    return "odd"
})
// {"odd": [1, 3, 5], "even": [2, 4]}

// 分块
slicesx.Chunk(nums, 2) // [[1, 2], [3, 4], [5]]

// 分区
evens, odds := slicesx.Partition(nums, func(n int) bool { return n%2 == 0 })

// 查找
val, ok := slicesx.Find(nums, func(n int) bool { return n > 3 })
idx := slicesx.FindIndex(nums, func(n int) bool { return n > 3 })

// 判断
slicesx.Any(nums, func(n int) bool { return n > 3 })  // true
slicesx.All(nums, func(n int) bool { return n > 0 })  // true
slicesx.None(nums, func(n int) bool { return n < 0 }) // true

// 其他
slicesx.Flatten([][]int{{1, 2}, {3, 4}}) // [1, 2, 3, 4]
slicesx.Compact([]string{"a", "", "b"})  // ["a", "b"]
slicesx.Take(nums, 3)                     // [1, 2, 3]
slicesx.Skip(nums, 2)                     // [3, 4, 5]
slicesx.First(nums)                       // 1, true
slicesx.Last(nums)                        // 5, true

// 键值对操作
pairs := slicesx.Zip([]string{"a", "b"}, []int{1, 2})
m := slicesx.ToMap(pairs) // {"a": 1, "b": 2}
m2 := slicesx.KeyBy(users, func(u User) int { return u.ID })
```

### mapsx - Map 操作

Map 操作工具函数。

```go
import "github.com/Tsukikage7/servex/v2/collections/mapsx"

m := map[string]int{"a": 1, "b": 2, "c": 3}

// 基本操作
mapsx.Keys(m)   // ["a", "b", "c"]（顺序不确定）
mapsx.Values(m) // [1, 2, 3]
mapsx.Clone(m)  // 浅拷贝

// 合并
m1 := map[string]int{"a": 1}
m2 := map[string]int{"b": 2, "a": 10}
mapsx.Merge(m1, m2) // {"a": 10, "b": 2}

// 过滤
mapsx.Filter(m, func(k string, v int) bool { return v > 1 })
mapsx.FilterKeys(m, "a", "c")  // {"a": 1, "c": 3}
mapsx.OmitKeys(m, "a")         // {"b": 2, "c": 3}

// 转换
mapsx.MapKeys(m, strings.ToUpper)
mapsx.MapValues(m, func(v int) int { return v * 10 })
mapsx.Invert(m) // {1: "a", 2: "b", 3: "c"}

// 比较
mapsx.Equal(m1, m2)
mapsx.Diff(m1, m2) // added, removed, changed

// 获取
mapsx.GetOrDefault(m, "x", 100)        // 100
mapsx.GetOrPut(m, "x", 100)            // 100（并写入 m）
mapsx.GetOrCompute(m, "x", func() int { return expensiveLoad() })

// 判断
mapsx.ContainsKey(m, "a")   // true
mapsx.ContainsValue(m, 1)   // true
mapsx.Any(m, func(k string, v int) bool { return v > 2 })
mapsx.All(m, func(k string, v int) bool { return v > 0 })
```

## 性能特性

| 数据结构      | Get/Contains | Put/Add  | Remove   | 有序 | 线程安全 |
| ------------- | ------------ | -------- | -------- | ---- | -------- |
| TreeMap       | O(log n)     | O(log n) | O(log n) | Y    | N        |
| TreeSet       | O(log n)     | O(log n) | O(log n) | Y    | N        |
| HashSet       | O(1)         | O(1)     | O(1)     | N    | N        |
| LRUCache      | O(1)         | O(1)     | O(1)     | N    | Y        |
| PriorityQueue | O(1) peek    | O(log n) | O(log n) | -    | N        |
| Deque         | O(1)         | O(1)     | O(1)     | Y    | N        |

## 测试

```bash
go test ./collections/...
```
