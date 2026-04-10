package treeset

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TreeSetTestSuite struct {
	suite.Suite
}

func TestTreeSetSuite(t *testing.T) {
	suite.Run(t, new(TreeSetTestSuite))
}

func (s *TreeSetTestSuite) TestNew() {
	ts := NewOrdered[int]()
	s.NotNil(ts)
	s.Equal(0, ts.Len())
	s.True(ts.IsEmpty())
}

func (s *TreeSetTestSuite) TestFromSlice() {
	ts := FromSlice([]int{3, 1, 2, 1})
	s.Equal(3, ts.Len())
	s.Equal([]int{1, 2, 3}, ts.ToSlice())
}

func (s *TreeSetTestSuite) TestAdd() {
	ts := NewOrdered[int]()
	ts.Add(3, 1, 2)

	s.Equal(3, ts.Len())
	s.Equal([]int{1, 2, 3}, ts.ToSlice())
}

func (s *TreeSetTestSuite) TestAddDuplicate() {
	ts := NewOrdered[int]()
	ts.Add(1, 1, 1)

	s.Equal(1, ts.Len())
}

func (s *TreeSetTestSuite) TestRemove() {
	ts := FromSlice([]int{1, 2, 3})
	ts.Remove(2)

	s.Equal(2, ts.Len())
	s.False(ts.Contains(2))
}

func (s *TreeSetTestSuite) TestContains() {
	ts := FromSlice([]int{1, 2, 3})

	s.True(ts.Contains(1))
	s.True(ts.Contains(2))
	s.True(ts.Contains(3))
	s.False(ts.Contains(4))
}

func (s *TreeSetTestSuite) TestClear() {
	ts := FromSlice([]int{1, 2, 3})
	ts.Clear()

	s.Equal(0, ts.Len())
	s.True(ts.IsEmpty())
}

func (s *TreeSetTestSuite) TestFirstAndLast() {
	ts := NewOrdered[int]()

	_, ok := ts.First()
	s.False(ok)
	_, ok = ts.Last()
	s.False(ok)

	ts.Add(3, 1, 2)

	first, ok := ts.First()
	s.True(ok)
	s.Equal(1, first)

	last, ok := ts.Last()
	s.True(ok)
	s.Equal(3, last)
}

func (s *TreeSetTestSuite) TestRange() {
	ts := FromSlice([]int{3, 1, 2})

	var items []int
	ts.Range(func(item int) bool {
		items = append(items, item)
		return true
	})
	s.Equal([]int{1, 2, 3}, items)
}

func (s *TreeSetTestSuite) TestClone() {
	ts := FromSlice([]int{1, 2, 3})
	clone := ts.Clone()

	clone.Add(4)
	s.Equal(3, ts.Len())
	s.Equal(4, clone.Len())
}

func (s *TreeSetTestSuite) TestUnion() {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{2, 3, 4})

	union := a.Union(b)
	s.Equal([]int{1, 2, 3, 4}, union.ToSlice())
}

func (s *TreeSetTestSuite) TestIntersection() {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{2, 3, 4})

	intersection := a.Intersection(b)
	s.Equal([]int{2, 3}, intersection.ToSlice())
}

func (s *TreeSetTestSuite) TestDifference() {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{2, 3, 4})

	diff := a.Difference(b)
	s.Equal([]int{1}, diff.ToSlice())
}

func (s *TreeSetTestSuite) TestIsSubset() {
	a := FromSlice([]int{1, 2})
	b := FromSlice([]int{1, 2, 3})

	s.True(a.IsSubset(b))
	s.False(b.IsSubset(a))
}

func (s *TreeSetTestSuite) TestIsSuperset() {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{1, 2})

	s.True(a.IsSuperset(b))
	s.False(b.IsSuperset(a))
}

func (s *TreeSetTestSuite) TestEqual() {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{3, 2, 1})
	c := FromSlice([]int{1, 2})

	s.True(a.Equal(b))
	s.False(a.Equal(c))
}

func (s *TreeSetTestSuite) TestStringSet() {
	ts := NewOrdered[string]()
	ts.Add("banana", "apple", "cherry")

	s.Equal([]string{"apple", "banana", "cherry"}, ts.ToSlice())
}

func (s *TreeSetTestSuite) TestAll() {
	ts := FromSlice([]int{3, 1, 2})
	var items []int
	for item := range ts.All() {
		items = append(items, item)
	}
	s.Equal([]int{1, 2, 3}, items)
}

func (s *TreeSetTestSuite) TestAllEmpty() {
	ts := NewOrdered[int]()
	count := 0
	for range ts.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *TreeSetTestSuite) TestAllEarlyBreak() {
	ts := FromSlice([]int{1, 2, 3, 4, 5})
	var items []int
	for item := range ts.All() {
		items = append(items, item)
		if item == 3 {
			break
		}
	}
	s.Equal([]int{1, 2, 3}, items)
}

func (s *TreeSetTestSuite) TestBackward() {
	ts := FromSlice([]int{3, 1, 2})
	var items []int
	for item := range ts.Backward() {
		items = append(items, item)
	}
	s.Equal([]int{3, 2, 1}, items)
}

func (s *TreeSetTestSuite) TestBackwardEmpty() {
	ts := NewOrdered[int]()
	count := 0
	for range ts.Backward() {
		count++
	}
	s.Equal(0, count)
}
