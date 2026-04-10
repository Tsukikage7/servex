package multimap

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MultiMapTestSuite struct {
	suite.Suite
}

func TestMultiMapSuite(t *testing.T) {
	suite.Run(t, new(MultiMapTestSuite))
}

func (s *MultiMapTestSuite) TestPutAndGet() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("a", 2)
	m.Put("b", 3)

	s.Equal([]int{1, 2}, m.Get("a"))
	s.Equal([]int{3}, m.Get("b"))
}

func (s *MultiMapTestSuite) TestGetMissing() {
	m := New[string, int]()
	s.Nil(m.Get("missing"))
}

func (s *MultiMapTestSuite) TestPutAll() {
	m := New[string, int]()
	m.PutAll("a", 1, 2, 3)
	s.Equal([]int{1, 2, 3}, m.Get("a"))
	s.Equal(3, m.Len())
}

func (s *MultiMapTestSuite) TestRemove() {
	m := New[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	ok := m.Remove("a")
	s.True(ok)
	s.False(m.ContainsKey("a"))
	s.Equal(1, m.Len())
}

func (s *MultiMapTestSuite) TestRemoveMissing() {
	m := New[string, int]()
	ok := m.Remove("missing")
	s.False(ok)
}

func (s *MultiMapTestSuite) TestRemoveValue() {
	m := New[string, int]()
	m.PutAll("a", 1, 2, 3)

	ok := RemoveValue(m, "a", 2)
	s.True(ok)
	s.Equal([]int{1, 3}, m.Get("a"))
	s.Equal(2, m.Len())
}

func (s *MultiMapTestSuite) TestRemoveValue_LastOne() {
	m := New[string, int]()
	m.Put("a", 1)

	ok := RemoveValue(m, "a", 1)
	s.True(ok)
	s.False(m.ContainsKey("a"))
	s.Equal(0, m.Len())
}

func (s *MultiMapTestSuite) TestRemoveValueMissing() {
	m := New[string, int]()
	m.Put("a", 1)
	ok := RemoveValue(m, "a", 99)
	s.False(ok)
}

func (s *MultiMapTestSuite) TestContainsKey() {
	m := New[string, int]()
	m.Put("a", 1)
	s.True(m.ContainsKey("a"))
	s.False(m.ContainsKey("b"))
}

func (s *MultiMapTestSuite) TestKeys() {
	m := New[string, int]()
	m.Put("b", 2)
	m.Put("a", 1)
	m.Put("c", 3)

	keys := m.Keys()
	sort.Strings(keys)
	s.Equal([]string{"a", "b", "c"}, keys)
}

func (s *MultiMapTestSuite) TestValues() {
	m := New[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	vals := m.Values()
	sort.Ints(vals)
	s.Equal([]int{1, 2, 3}, vals)
}

func (s *MultiMapTestSuite) TestLen() {
	m := New[string, int]()
	s.Equal(0, m.Len())
	m.PutAll("a", 1, 2, 3)
	s.Equal(3, m.Len())
	m.Put("b", 4)
	s.Equal(4, m.Len())
}

func (s *MultiMapTestSuite) TestKeyLen() {
	m := New[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)
	s.Equal(2, m.KeyLen())
}

func (s *MultiMapTestSuite) TestRange() {
	m := New[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	total := 0
	m.Range(func(_ string, vals []int) bool {
		total += len(vals)
		return true
	})
	s.Equal(3, total)
}

func (s *MultiMapTestSuite) TestRangeEarlyStop() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	count := 0
	m.Range(func(_ string, _ []int) bool {
		count++
		return count < 2
	})
	s.Equal(2, count)
}

func (s *MultiMapTestSuite) TestClear() {
	m := New[string, int]()
	m.PutAll("a", 1, 2, 3)
	m.Clear()
	s.Equal(0, m.Len())
	s.Equal(0, m.KeyLen())
	s.Nil(m.Get("a"))
}

func (s *MultiMapTestSuite) TestAll() {
	m := New[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	total := 0
	for _, vals := range m.All() {
		total += len(vals)
	}
	s.Equal(3, total)
}

func (s *MultiMapTestSuite) TestAllEmpty() {
	m := New[string, int]()
	count := 0
	for range m.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *MultiMapTestSuite) TestAllEarlyBreak() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	count := 0
	for range m.All() {
		count++
		if count == 1 {
			break
		}
	}
	s.Equal(1, count)
}
