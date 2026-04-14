# servex 集合库

## blockingqueue -- 阻塞队列

```go
import "github.com/Tsukikage7/servex/v2/collections/blockingqueue"

// 创建容量为 100 的阻塞队列
q := blockingqueue.New[string](100)

// 入队（满时阻塞，直到有空位或 ctx 取消）
err := q.Enqueue(ctx, "task-1")

// 出队（空时阻塞，直到有元素或 ctx 取消）
item, err := q.Dequeue(ctx)

q.Len()     // 当前元素数
q.IsFull()  // 是否已满
q.IsEmpty() // 是否为空
```

**关键特性：**
- 基于环形缓冲区 + 双信号量，入队/出队均为 O(1)
- 满时 Enqueue 阻塞，空时 Dequeue 阻塞
- 通过 context 取消解除阻塞

## delayqueue -- 延迟队列

```go
import "github.com/Tsukikage7/servex/v2/collections/delayqueue"

// 实现 Delayable 接口
type Task struct {
    ID      string
    RunAt   time.Time
}
func (t Task) Delay() time.Duration { return time.Until(t.RunAt) }

// 创建延迟队列
dq := delayqueue.New[Task](0)

// 入队（立即返回）
dq.Enqueue(ctx, Task{ID: "1", RunAt: time.Now().Add(5 * time.Second)})

// 出队（阻塞直到最早的元素到期）
task, err := dq.Dequeue(ctx)
```

**关键特性：**
- 元素必须实现 `Delayable` 接口（`Delay() time.Duration`）
- 基于优先队列，最早到期的元素先出队
- Dequeue 阻塞直到有元素到期或 ctx 取消

## deque -- 双端队列

```go
import "github.com/Tsukikage7/servex/v2/collections/deque"

dq := deque.New[int]()          // 空双端队列
dq = deque.From([]int{1, 2, 3}) // 从切片创建

dq.PushFront(0)   // 头部添加
dq.PushBack(4)    // 尾部添加

v, ok := dq.PopFront()  // 头部弹出: 0
v, ok = dq.PopBack()    // 尾部弹出: 4

v, ok = dq.PeekFront()  // 查看头部（不移除）
v, ok = dq.PeekBack()   // 查看尾部（不移除）

dq.At(0)  // 按索引访问
dq.Len()  // 元素数量
```

**关键特性：**
- 基于环形缓冲区，PushFront/PushBack/PopFront/PopBack 均 O(1)
- 自动扩容和缩容

## hashset -- 无序集合

```go
import "github.com/Tsukikage7/servex/v2/collections/hashset"

s := hashset.New(1, 2, 3)       // 创建并初始化
s = hashset.FromSlice([]int{1, 2, 3})

s.Add(4, 5)
s.Remove(1)
s.Contains(2)  // true
s.Len()        // 元素数
s.ToSlice()    // 转为切片

// 集合运算
other := hashset.New(2, 3, 6)
union := s.Union(other)        // 并集
inter := s.Intersection(other) // 交集

// 遍历
s.Range(func(item int) bool {
    fmt.Println(item)
    return true // 返回 false 停止
})
```

## linkedmap -- 有序 Map（按插入顺序）

```go
import "github.com/Tsukikage7/servex/v2/collections/linkedmap"

m := linkedmap.New[string, int]()
m.Put("b", 2)
m.Put("a", 1)
m.Put("c", 3)

val, ok := m.Get("a")    // 1, true
m.ContainsKey("b")        // true
m.Remove("c")

m.Keys()   // ["b", "a"] — 按插入顺序
m.Values() // [2, 1]

m.Range(func(k string, v int) bool {
    fmt.Printf("%s=%d\n", k, v)
    return true
})
```

## lrucache -- LRU 缓存

```go
import "github.com/Tsukikage7/servex/v2/collections/lrucache"

cache := lrucache.New[string, int](100) // 容量 100

cache.Put("a", 1)
cache.Put("b", 2)

val, ok := cache.Get("a")  // 1, true（移到最近使用位置）
val, ok = cache.Peek("b")  // 2, true（不影响 LRU 顺序）

// 自动加载
val = cache.GetOrPut("c", func() int {
    return expensiveCompute()
})
```

**关键特性：**
- 哈希表 + 双向链表，Get/Put 均 O(1)
- 满时自动淘汰最久未使用的元素
- 线程安全（内置 sync.RWMutex）

## mapsx -- Map 工具函数

```go
import "github.com/Tsukikage7/servex/v2/collections/mapsx"

m := map[string]int{"a": 1, "b": 2, "c": 3}

mapsx.Keys(m)   // ["a", "b", "c"]（顺序不确定）
mapsx.Values(m) // [1, 2, 3]

// 合并（后面覆盖前面）
merged := mapsx.Merge(m, map[string]int{"a": 10, "d": 4})
// {"a": 10, "b": 2, "c": 3, "d": 4}

// 过滤
filtered := mapsx.Filter(m, func(k string, v int) bool { return v > 1 })
// {"b": 2, "c": 3}

// 只保留指定键
sub := mapsx.FilterKeys(m, "a", "c") // {"a": 1, "c": 3}

// 键值对互转
entries := mapsx.Entries(m)
m2 := mapsx.FromEntries(entries)
```

## multimap -- 一对多映射

```go
import "github.com/Tsukikage7/servex/v2/collections/multimap"

mm := multimap.New[string, int]()
mm.Put("tag", 1)
mm.Put("tag", 2)
mm.PutAll("tag", 3, 4)

mm.Get("tag")        // [1, 2, 3, 4]
mm.ContainsKey("tag") // true
mm.Len()              // 4（总值数）
mm.KeyLen()           // 1（键数）

mm.Remove("tag")     // 移除整个键

// 移除特定值（V 需 comparable）
multimap.RemoveValue(mm, "tag", 2)
```

## priorityqueue -- 优先队列

```go
import "github.com/Tsukikage7/servex/v2/collections/priorityqueue"

// 最小堆
pq := priorityqueue.NewMin[int]()
pq.Push(3, 1, 2)
val, ok := pq.Pop()  // 1
val, ok = pq.Peek()  // 2（不弹出）

// 最大堆
pq := priorityqueue.NewMax[int]()

// 自定义优先级
pq := priorityqueue.New(func(a, b Task) bool {
    return a.Priority > b.Priority
})

// 线程安全版本
cpq := priorityqueue.NewConcurrentMin[int]()
cpq.Push(3, 1, 2)
val, ok = cpq.Pop()  // 1
```

**关键特性：**
- 基于二叉堆，Push/Pop 均 O(log n)
- `NewMin` / `NewMax` 快捷创建
- `NewConcurrent*` 线程安全版本

## slicesx -- 切片工具函数

```go
import "github.com/Tsukikage7/servex/v2/collections/slicesx"

nums := []int{1, 2, 3, 4, 5}

// 过滤
evens := slicesx.Filter(nums, func(n int) bool { return n%2 == 0 })
// [2, 4]

// 转换
strs := slicesx.Map(nums, strconv.Itoa) // ["1", "2", "3", "4", "5"]

// 归约
sum := slicesx.Reduce(nums, 0, func(acc, n int) int { return acc + n })
// 15

// 去重
slicesx.Unique([]int{1, 2, 2, 3, 1}) // [1, 2, 3]

// 按键去重
slicesx.UniqueBy(users, func(u User) int { return u.ID })

// 分组
groups := slicesx.GroupBy(nums, func(n int) string {
    if n%2 == 0 { return "even" }
    return "odd"
})

// 分块
chunks := slicesx.Chunk(nums, 2) // [[1,2], [3,4], [5]]
```

## treemap -- 有序 Map（按键排序）

```go
import "github.com/Tsukikage7/servex/v2/collections/treemap"

// 使用内置类型的默认比较
tm := treemap.NewOrdered[int, string]()
tm.Put(3, "three")
tm.Put(1, "one")
tm.Put(2, "two")

val, ok := tm.Get(1)  // "one", true
tm.Keys()             // [1, 2, 3] — 按键排序

// 自定义比较器
tm2 := treemap.New[string, int](treemap.ReverseCompare[string])

// 范围遍历
tm.Range(func(k int, v string) bool {
    fmt.Printf("%d -> %s\n", k, v)
    return true
})

// 首尾元素
first, _ := tm.FirstKey()  // 1
last, _ := tm.LastKey()    // 3
```

**关键特性：**
- 基于红黑树，Put/Get/Remove 均 O(log n)
- 内置比较器：`OrderedCompare`、`ReverseCompare`、`TimeCompare`

## treeset -- 有序集合

```go
import "github.com/Tsukikage7/servex/v2/collections/treeset"

ts := treeset.NewOrdered[int]()
ts.Add(3, 1, 2)

ts.Contains(2) // true
ts.ToSlice()   // [1, 2, 3] — 按排序顺序
ts.First()     // 1
ts.Last()      // 3

// 从切片创建
ts = treeset.FromSlice([]int{5, 3, 1})

// 集合运算
other := treeset.NewOrdered[int]()
other.Add(2, 4)
union := ts.Union(other)        // 并集
inter := ts.Intersection(other) // 交集
```
