package hashset

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HashSetTestSuite struct {
	suite.Suite
}

func TestHashSetSuite(t *testing.T) {
	suite.Run(t, new(HashSetTestSuite))
}

func (s *HashSetTestSuite) TestNew() {
	hs := New[int]()
	s.NotNil(hs)
	s.Equal(0, hs.Len())
	s.True(hs.IsEmpty())
}

func (s *HashSetTestSuite) TestNewWithItems() {
	hs := New(1, 2, 3)
	s.Equal(3, hs.Len())
}

func (s *HashSetTestSuite) TestFromSlice() {
	hs := FromSlice([]int{1, 2, 2, 3})
	s.Equal(3, hs.Len())
}

func (s *HashSetTestSuite) TestAdd() {
	hs := New[int]()
	hs.Add(1, 2, 3)

	s.Equal(3, hs.Len())
	s.True(hs.Contains(1))
	s.True(hs.Contains(2))
	s.True(hs.Contains(3))
}

func (s *HashSetTestSuite) TestAddDuplicate() {
	hs := New[int]()
	hs.Add(1, 1, 1)

	s.Equal(1, hs.Len())
}

func (s *HashSetTestSuite) TestRemove() {
	hs := New(1, 2, 3)
	hs.Remove(2)

	s.Equal(2, hs.Len())
	s.False(hs.Contains(2))
}

func (s *HashSetTestSuite) TestContains() {
	hs := New(1, 2, 3)

	s.True(hs.Contains(1))
	s.True(hs.Contains(2))
	s.True(hs.Contains(3))
	s.False(hs.Contains(4))
}

func (s *HashSetTestSuite) TestClear() {
	hs := New(1, 2, 3)
	hs.Clear()

	s.Equal(0, hs.Len())
	s.True(hs.IsEmpty())
}

func (s *HashSetTestSuite) TestToSlice() {
	hs := New(3, 1, 2)
	slice := hs.ToSlice()

	s.Len(slice, 3)
	sort.Ints(slice)
	s.Equal([]int{1, 2, 3}, slice)
}

func (s *HashSetTestSuite) TestRange() {
	hs := New(1, 2, 3)

	var items []int
	hs.Range(func(item int) bool {
		items = append(items, item)
		return true
	})
	s.Len(items, 3)
}

func (s *HashSetTestSuite) TestRangeStop() {
	hs := New(1, 2, 3, 4, 5)

	count := 0
	hs.Range(func(item int) bool {
		count++
		return count < 3
	})
	s.Equal(3, count)
}

func (s *HashSetTestSuite) TestClone() {
	hs := New(1, 2, 3)
	clone := hs.Clone()

	clone.Add(4)
	s.Equal(3, hs.Len())
	s.Equal(4, clone.Len())
}

func (s *HashSetTestSuite) TestUnion() {
	a := New(1, 2, 3)
	b := New(2, 3, 4)

	union := a.Union(b)
	s.Equal(4, union.Len())
	s.True(union.Contains(1))
	s.True(union.Contains(2))
	s.True(union.Contains(3))
	s.True(union.Contains(4))
}

func (s *HashSetTestSuite) TestIntersection() {
	a := New(1, 2, 3)
	b := New(2, 3, 4)

	intersection := a.Intersection(b)
	s.Equal(2, intersection.Len())
	s.True(intersection.Contains(2))
	s.True(intersection.Contains(3))
}

func (s *HashSetTestSuite) TestDifference() {
	a := New(1, 2, 3)
	b := New(2, 3, 4)

	diff := a.Difference(b)
	s.Equal(1, diff.Len())
	s.True(diff.Contains(1))
}

func (s *HashSetTestSuite) TestSymmetricDifference() {
	a := New(1, 2, 3)
	b := New(2, 3, 4)

	symDiff := a.SymmetricDifference(b)
	s.Equal(2, symDiff.Len())
	s.True(symDiff.Contains(1))
	s.True(symDiff.Contains(4))
}

func (s *HashSetTestSuite) TestIsSubset() {
	a := New(1, 2)
	b := New(1, 2, 3)

	s.True(a.IsSubset(b))
	s.False(b.IsSubset(a))
}

func (s *HashSetTestSuite) TestIsSuperset() {
	a := New(1, 2, 3)
	b := New(1, 2)

	s.True(a.IsSuperset(b))
	s.False(b.IsSuperset(a))
}

func (s *HashSetTestSuite) TestEqual() {
	a := New(1, 2, 3)
	b := New(3, 2, 1)
	c := New(1, 2)

	s.True(a.Equal(b))
	s.False(a.Equal(c))
}

func (s *HashSetTestSuite) TestIsDisjoint() {
	a := New(1, 2)
	b := New(3, 4)
	c := New(2, 3)

	s.True(a.IsDisjoint(b))
	s.False(a.IsDisjoint(c))
}

func (s *HashSetTestSuite) TestStringSet() {
	hs := New("apple", "banana", "cherry")

	s.Equal(3, hs.Len())
	s.True(hs.Contains("apple"))
	s.False(hs.Contains("grape"))
}

func (s *HashSetTestSuite) TestAll() {
	hs := New(1, 2, 3)
	var items []int
	for item := range hs.All() {
		items = append(items, item)
	}
	sort.Ints(items)
	s.Equal([]int{1, 2, 3}, items)
}

func (s *HashSetTestSuite) TestAllEmpty() {
	hs := New[int]()
	count := 0
	for range hs.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *HashSetTestSuite) TestAllEarlyBreak() {
	hs := New(1, 2, 3, 4, 5)
	count := 0
	for range hs.All() {
		count++
		if count == 2 {
			break
		}
	}
	s.Equal(2, count)
}
