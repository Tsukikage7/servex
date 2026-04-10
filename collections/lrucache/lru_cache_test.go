package lrucache

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type LRUCacheTestSuite struct {
	suite.Suite
}

func TestLRUCacheSuite(t *testing.T) {
	suite.Run(t, new(LRUCacheTestSuite))
}

func (s *LRUCacheTestSuite) TestNew() {
	cache := New[string, int](10)
	s.NotNil(cache)
	s.Equal(0, cache.Len())
	s.Equal(10, cache.Capacity())
}

func (s *LRUCacheTestSuite) TestNewZeroCapacity() {
	cache := New[string, int](0)
	s.Equal(1, cache.Capacity())
}

func (s *LRUCacheTestSuite) TestPutAndGet() {
	cache := New[string, int](10)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	val, ok := cache.Get("a")
	s.True(ok)
	s.Equal(1, val)

	val, ok = cache.Get("b")
	s.True(ok)
	s.Equal(2, val)

	_, ok = cache.Get("d")
	s.False(ok)
}

func (s *LRUCacheTestSuite) TestPutUpdate() {
	cache := New[string, int](10)

	cache.Put("a", 1)
	cache.Put("a", 100)

	val, _ := cache.Get("a")
	s.Equal(100, val)
	s.Equal(1, cache.Len())
}

func (s *LRUCacheTestSuite) TestEviction() {
	cache := New[string, int](3)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// 缓存满，添加新元素应该淘汰 "a"
	cache.Put("d", 4)

	s.Equal(3, cache.Len())
	s.False(cache.Contains("a"))
	s.True(cache.Contains("b"))
	s.True(cache.Contains("c"))
	s.True(cache.Contains("d"))
}

func (s *LRUCacheTestSuite) TestLRUOrder() {
	cache := New[string, int](3)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// 访问 "a"，使其成为最近使用
	cache.Get("a")

	// 添加新元素，应该淘汰 "b"（最久未使用）
	cache.Put("d", 4)

	s.False(cache.Contains("b"))
	s.True(cache.Contains("a"))
	s.True(cache.Contains("c"))
	s.True(cache.Contains("d"))
}

func (s *LRUCacheTestSuite) TestPeek() {
	cache := New[string, int](3)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// Peek 不应该影响 LRU 顺序
	val, ok := cache.Peek("a")
	s.True(ok)
	s.Equal(1, val)

	// 添加新元素，"a" 仍然是最久未使用（因为用的是 Peek）
	cache.Put("d", 4)

	s.False(cache.Contains("a"))
}

func (s *LRUCacheTestSuite) TestContains() {
	cache := New[string, int](10)
	cache.Put("a", 1)

	s.True(cache.Contains("a"))
	s.False(cache.Contains("b"))
}

func (s *LRUCacheTestSuite) TestRemove() {
	cache := New[string, int](10)
	cache.Put("a", 1)
	cache.Put("b", 2)

	ok := cache.Remove("a")
	s.True(ok)
	s.Equal(1, cache.Len())
	s.False(cache.Contains("a"))

	ok = cache.Remove("c")
	s.False(ok)
}

func (s *LRUCacheTestSuite) TestClear() {
	cache := New[string, int](10)
	cache.Put("a", 1)
	cache.Put("b", 2)

	cache.Clear()
	s.Equal(0, cache.Len())
	s.False(cache.Contains("a"))
}

func (s *LRUCacheTestSuite) TestKeys() {
	cache := New[string, int](10)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)
	cache.Get("a") // 移动 "a" 到最近

	keys := cache.Keys()
	s.Equal([]string{"a", "c", "b"}, keys)
}

func (s *LRUCacheTestSuite) TestResize() {
	cache := New[string, int](5)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)
	cache.Put("d", 4)
	cache.Put("e", 5)

	// 缩小容量
	cache.Resize(3)

	s.Equal(3, cache.Len())
	s.Equal(3, cache.Capacity())

	// 最久未使用的 "a", "b" 应该被淘汰
	s.False(cache.Contains("a"))
	s.False(cache.Contains("b"))
	s.True(cache.Contains("c"))
	s.True(cache.Contains("d"))
	s.True(cache.Contains("e"))
}

func (s *LRUCacheTestSuite) TestGetOrPut() {
	cache := New[string, int](10)

	// 不存在，调用 loader
	callCount := 0
	val := cache.GetOrPut("a", func() int {
		callCount++
		return 100
	})
	s.Equal(100, val)
	s.Equal(1, callCount)

	// 已存在，不调用 loader
	val = cache.GetOrPut("a", func() int {
		callCount++
		return 200
	})
	s.Equal(100, val)
	s.Equal(1, callCount)
}

func (s *LRUCacheTestSuite) TestConcurrency() {
	cache := New[int, int](100)
	var wg sync.WaitGroup

	// 并发写
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			cache.Put(i, i*10)
		})
	}

	// 并发读
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			cache.Get(i)
		})
	}

	wg.Wait()
	s.LessOrEqual(cache.Len(), 100)
}

func (s *LRUCacheTestSuite) TestIntKey() {
	cache := New[int, string](10)

	cache.Put(1, "one")
	cache.Put(2, "two")

	val, ok := cache.Get(1)
	s.True(ok)
	s.Equal("one", val)
}

func (s *LRUCacheTestSuite) TestAll() {
	cache := New[string, int](10)
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// 最近使用在前：c, b, a
	var keys []string
	var vals []int
	for k, v := range cache.All() {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	s.Equal([]string{"c", "b", "a"}, keys)
	s.Equal([]int{3, 2, 1}, vals)
}

func (s *LRUCacheTestSuite) TestAllAfterAccess() {
	cache := New[string, int](10)
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)
	cache.Get("a") // 访问 a，使其成为最近使用

	// 最近使用在前：a, c, b
	var keys []string
	for k, _ := range cache.All() {
		keys = append(keys, k)
	}
	s.Equal([]string{"a", "c", "b"}, keys)
}

func (s *LRUCacheTestSuite) TestAllEmpty() {
	cache := New[string, int](10)
	count := 0
	for range cache.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *LRUCacheTestSuite) TestAllEarlyBreak() {
	cache := New[string, int](10)
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	count := 0
	for range cache.All() {
		count++
		if count == 2 {
			break
		}
	}
	s.Equal(2, count)
}
