package linkedmap

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type LinkedMapTestSuite struct {
	suite.Suite
}

func TestLinkedMapSuite(t *testing.T) {
	suite.Run(t, new(LinkedMapTestSuite))
}

func (s *LinkedMapTestSuite) TestPutAndGet() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	v, ok := m.Get("a")
	s.True(ok)
	s.Equal(1, v)

	v, ok = m.Get("b")
	s.True(ok)
	s.Equal(2, v)
}

func (s *LinkedMapTestSuite) TestGetMissing() {
	m := New[string, int]()
	v, ok := m.Get("missing")
	s.False(ok)
	s.Equal(0, v)
}

func (s *LinkedMapTestSuite) TestUpdateKeepsOrder() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("a", 10) // 更新不改变顺序

	s.Equal([]string{"a", "b"}, m.Keys())
	v, _ := m.Get("a")
	s.Equal(10, v)
}

func (s *LinkedMapTestSuite) TestInsertionOrder() {
	m := New[string, int]()
	keys := []string{"c", "a", "b", "z"}
	for i, k := range keys {
		m.Put(k, i)
	}
	s.Equal(keys, m.Keys())
	s.Equal([]int{0, 1, 2, 3}, m.Values())
}

func (s *LinkedMapTestSuite) TestRemove() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	ok := m.Remove("b")
	s.True(ok)
	s.Equal([]string{"a", "c"}, m.Keys())
	s.Equal(2, m.Len())
}

func (s *LinkedMapTestSuite) TestRemoveMissing() {
	m := New[string, int]()
	ok := m.Remove("missing")
	s.False(ok)
}

func (s *LinkedMapTestSuite) TestContainsKey() {
	m := New[string, int]()
	m.Put("a", 1)
	s.True(m.ContainsKey("a"))
	s.False(m.ContainsKey("b"))
}

func (s *LinkedMapTestSuite) TestLen() {
	m := New[string, int]()
	s.Equal(0, m.Len())
	m.Put("a", 1)
	s.Equal(1, m.Len())
	m.Remove("a")
	s.Equal(0, m.Len())
}

func (s *LinkedMapTestSuite) TestRange() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	var keys []string
	var vals []int
	m.Range(func(k string, v int) bool {
		keys = append(keys, k)
		vals = append(vals, v)
		return true
	})
	s.Equal([]string{"a", "b", "c"}, keys)
	s.Equal([]int{1, 2, 3}, vals)
}

func (s *LinkedMapTestSuite) TestRangeEarlyStop() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	count := 0
	m.Range(func(_ string, _ int) bool {
		count++
		return count < 2
	})
	s.Equal(2, count)
}

func (s *LinkedMapTestSuite) TestClear() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Clear()
	s.Equal(0, m.Len())
	s.Empty(m.Keys())
	s.Empty(m.Values())
}

func (s *LinkedMapTestSuite) TestAll() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	var keys []string
	var vals []int
	for k, v := range m.All() {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	s.Equal([]string{"a", "b", "c"}, keys)
	s.Equal([]int{1, 2, 3}, vals)
}

func (s *LinkedMapTestSuite) TestAllEmpty() {
	m := New[string, int]()
	count := 0
	for range m.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *LinkedMapTestSuite) TestAllEarlyBreak() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	var keys []string
	for k := range m.All() {
		keys = append(keys, k)
		if k == "b" {
			break
		}
	}
	s.Equal([]string{"a", "b"}, keys)
}

func (s *LinkedMapTestSuite) TestBackward() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	var keys []string
	var vals []int
	for k, v := range m.Backward() {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	s.Equal([]string{"c", "b", "a"}, keys)
	s.Equal([]int{3, 2, 1}, vals)
}

func (s *LinkedMapTestSuite) TestBackwardEmpty() {
	m := New[string, int]()
	count := 0
	for range m.Backward() {
		count++
	}
	s.Equal(0, count)
}
