package treemap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TreeMapTestSuite struct {
	suite.Suite
}

func TestTreeMapSuite(t *testing.T) {
	suite.Run(t, new(TreeMapTestSuite))
}

func (s *TreeMapTestSuite) TestNew() {
	tm := NewOrdered[int, string]()
	s.NotNil(tm)
	s.Equal(0, tm.Len())
	s.True(tm.IsEmpty())
}

func (s *TreeMapTestSuite) TestPutAndGet() {
	tm := NewOrdered[int, string]()

	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	s.Equal(3, tm.Len())

	v, ok := tm.Get(1)
	s.True(ok)
	s.Equal("one", v)

	v, ok = tm.Get(2)
	s.True(ok)
	s.Equal("two", v)

	v, ok = tm.Get(3)
	s.True(ok)
	s.Equal("three", v)

	_, ok = tm.Get(4)
	s.False(ok)
}

func (s *TreeMapTestSuite) TestPutUpdate() {
	tm := NewOrdered[int, string]()

	tm.Put(1, "one")
	tm.Put(1, "ONE")

	s.Equal(1, tm.Len())
	v, _ := tm.Get(1)
	s.Equal("ONE", v)
}

func (s *TreeMapTestSuite) TestGetOrDefault() {
	tm := NewOrdered[int, string]()
	tm.Put(1, "one")

	s.Equal("one", tm.GetOrDefault(1, "default"))
	s.Equal("default", tm.GetOrDefault(2, "default"))
}

func (s *TreeMapTestSuite) TestRemove() {
	tm := NewOrdered[int, string]()
	tm.Put(1, "one")
	tm.Put(2, "two")
	tm.Put(3, "three")

	v, ok := tm.Remove(2)
	s.True(ok)
	s.Equal("two", v)
	s.Equal(2, tm.Len())
	s.False(tm.ContainsKey(2))

	_, ok = tm.Remove(4)
	s.False(ok)
}

func (s *TreeMapTestSuite) TestContainsKey() {
	tm := NewOrdered[int, string]()
	tm.Put(1, "one")

	s.True(tm.ContainsKey(1))
	s.False(tm.ContainsKey(2))
}

func (s *TreeMapTestSuite) TestClear() {
	tm := NewOrdered[int, string]()
	tm.Put(1, "one")
	tm.Put(2, "two")

	tm.Clear()
	s.Equal(0, tm.Len())
	s.True(tm.IsEmpty())
}

func (s *TreeMapTestSuite) TestKeys() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	keys := tm.Keys()
	s.Equal([]int{1, 2, 3}, keys)
}

func (s *TreeMapTestSuite) TestValues() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	values := tm.Values()
	s.Equal([]string{"one", "two", "three"}, values)
}

func (s *TreeMapTestSuite) TestEntries() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	entries := tm.Entries()
	s.Len(entries, 3)
	s.Equal(Entry[int, string]{Key: 1, Value: "one"}, entries[0])
	s.Equal(Entry[int, string]{Key: 2, Value: "two"}, entries[1])
	s.Equal(Entry[int, string]{Key: 3, Value: "three"}, entries[2])
}

func (s *TreeMapTestSuite) TestRange() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	var keys []int
	tm.Range(func(key int, value string) bool {
		keys = append(keys, key)
		return true
	})
	s.Equal([]int{1, 2, 3}, keys)
}

func (s *TreeMapTestSuite) TestRangeStop() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	var keys []int
	tm.Range(func(key int, value string) bool {
		keys = append(keys, key)
		return key < 2
	})
	s.Equal([]int{1, 2}, keys)
}

func (s *TreeMapTestSuite) TestFirstAndLast() {
	tm := NewOrdered[int, string]()

	_, ok := tm.FirstKey()
	s.False(ok)
	_, ok = tm.LastKey()
	s.False(ok)

	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	first, ok := tm.FirstKey()
	s.True(ok)
	s.Equal(1, first)

	last, ok := tm.LastKey()
	s.True(ok)
	s.Equal(3, last)

	firstEntry, ok := tm.First()
	s.True(ok)
	s.Equal(Entry[int, string]{Key: 1, Value: "one"}, firstEntry)

	lastEntry, ok := tm.Last()
	s.True(ok)
	s.Equal(Entry[int, string]{Key: 3, Value: "three"}, lastEntry)
}

func (s *TreeMapTestSuite) TestClone() {
	tm := NewOrdered[int, string]()
	tm.Put(1, "one")
	tm.Put(2, "two")

	clone := tm.Clone()
	s.Equal(2, clone.Len())

	clone.Put(3, "three")
	s.Equal(2, tm.Len())
	s.Equal(3, clone.Len())
}

func (s *TreeMapTestSuite) TestStringKey() {
	tm := NewOrdered[string, int]()
	tm.Put("banana", 2)
	tm.Put("apple", 1)
	tm.Put("cherry", 3)

	keys := tm.Keys()
	s.Equal([]string{"apple", "banana", "cherry"}, keys)
}

func (s *TreeMapTestSuite) TestReverseOrder() {
	tm := New[int, string](ReverseCompare[int])
	tm.Put(1, "one")
	tm.Put(2, "two")
	tm.Put(3, "three")

	keys := tm.Keys()
	s.Equal([]int{3, 2, 1}, keys)
}

func (s *TreeMapTestSuite) TestTimeKey() {
	tm := New[time.Time, string](TimeCompare)
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

	tm.Put(t2, "mid")
	tm.Put(t1, "start")
	tm.Put(t3, "end")

	keys := tm.Keys()
	s.Equal([]time.Time{t1, t2, t3}, keys)
}

func (s *TreeMapTestSuite) TestLargeDataset() {
	tm := NewOrdered[int, int]()
	n := 10000

	// 插入
	for i := n; i > 0; i-- {
		tm.Put(i, i*10)
	}
	s.Equal(n, tm.Len())

	// 验证顺序
	keys := tm.Keys()
	for i := 0; i < n; i++ {
		s.Equal(i+1, keys[i])
	}

	// 删除一半
	for i := 1; i <= n/2; i++ {
		tm.Remove(i * 2)
	}
	s.Equal(n/2, tm.Len())

	// 验证剩余
	for i := 1; i <= n; i++ {
		if i%2 == 0 {
			s.False(tm.ContainsKey(i))
		} else {
			s.True(tm.ContainsKey(i))
		}
	}
}

func (s *TreeMapTestSuite) TestAll() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	var keys []int
	var vals []string
	for k, v := range tm.All() {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	s.Equal([]int{1, 2, 3}, keys)
	s.Equal([]string{"one", "two", "three"}, vals)
}

func (s *TreeMapTestSuite) TestAllEmpty() {
	tm := NewOrdered[int, string]()
	count := 0
	for range tm.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *TreeMapTestSuite) TestAllEarlyBreak() {
	tm := NewOrdered[int, string]()
	tm.Put(1, "one")
	tm.Put(2, "two")
	tm.Put(3, "three")

	var keys []int
	for k := range tm.All() {
		keys = append(keys, k)
		if k == 2 {
			break
		}
	}
	s.Equal([]int{1, 2}, keys)
}

func (s *TreeMapTestSuite) TestBackward() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	var keys []int
	var vals []string
	for k, v := range tm.Backward() {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	s.Equal([]int{3, 2, 1}, keys)
	s.Equal([]string{"three", "two", "one"}, vals)
}

func (s *TreeMapTestSuite) TestBackwardEmpty() {
	tm := NewOrdered[int, string]()
	count := 0
	for range tm.Backward() {
		count++
	}
	s.Equal(0, count)
}

func (s *TreeMapTestSuite) TestRangeReverse() {
	tm := NewOrdered[int, string]()
	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	var keys []int
	tm.RangeReverse(func(k int, v string) bool {
		keys = append(keys, k)
		return true
	})
	s.Equal([]int{3, 2, 1}, keys)
}
