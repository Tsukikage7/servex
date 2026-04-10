package slicesx

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SlicesTestSuite struct {
	suite.Suite
}

func TestSlicesSuite(t *testing.T) {
	suite.Run(t, new(SlicesTestSuite))
}

func (s *SlicesTestSuite) TestFilter() {
	nums := []int{1, 2, 3, 4, 5}
	evens := Filter(nums, func(n int) bool { return n%2 == 0 })
	s.Equal([]int{2, 4}, evens)
}

func (s *SlicesTestSuite) TestFilterEmpty() {
	var nums []int
	result := Filter(nums, func(n int) bool { return true })
	s.Empty(result)
}

func (s *SlicesTestSuite) TestMap() {
	nums := []int{1, 2, 3}
	strs := Map(nums, strconv.Itoa)
	s.Equal([]string{"1", "2", "3"}, strs)
}

func (s *SlicesTestSuite) TestMapEmpty() {
	var nums []int
	result := Map(nums, strconv.Itoa)
	s.Empty(result)
}

func (s *SlicesTestSuite) TestReduce() {
	nums := []int{1, 2, 3, 4}
	sum := Reduce(nums, 0, func(acc, n int) int { return acc + n })
	s.Equal(10, sum)
}

func (s *SlicesTestSuite) TestReduceEmpty() {
	var nums []int
	sum := Reduce(nums, 100, func(acc, n int) int { return acc + n })
	s.Equal(100, sum)
}

func (s *SlicesTestSuite) TestUnique() {
	nums := []int{1, 2, 2, 3, 1, 4}
	unique := Unique(nums)
	s.Equal([]int{1, 2, 3, 4}, unique)
}

func (s *SlicesTestSuite) TestUniqueBy() {
	type User struct {
		ID   int
		Name string
	}
	users := []User{{1, "a"}, {2, "b"}, {1, "c"}}
	unique := UniqueBy(users, func(u User) int { return u.ID })
	s.Len(unique, 2)
	s.Equal("a", unique[0].Name)
	s.Equal("b", unique[1].Name)
}

func (s *SlicesTestSuite) TestGroupBy() {
	nums := []int{1, 2, 3, 4, 5}
	groups := GroupBy(nums, func(n int) string {
		if n%2 == 0 {
			return "even"
		}
		return "odd"
	})
	s.Equal([]int{1, 3, 5}, groups["odd"])
	s.Equal([]int{2, 4}, groups["even"])
}

func (s *SlicesTestSuite) TestChunk() {
	nums := []int{1, 2, 3, 4, 5}
	chunks := Chunk(nums, 2)
	s.Equal([][]int{{1, 2}, {3, 4}, {5}}, chunks)
}

func (s *SlicesTestSuite) TestChunkExact() {
	nums := []int{1, 2, 3, 4}
	chunks := Chunk(nums, 2)
	s.Equal([][]int{{1, 2}, {3, 4}}, chunks)
}

func (s *SlicesTestSuite) TestChunkInvalid() {
	nums := []int{1, 2, 3}
	s.Nil(Chunk(nums, 0))
	s.Nil(Chunk(nums, -1))
}

func (s *SlicesTestSuite) TestPartition() {
	nums := []int{1, 2, 3, 4, 5}
	evens, odds := Partition(nums, func(n int) bool { return n%2 == 0 })
	s.Equal([]int{2, 4}, evens)
	s.Equal([]int{1, 3, 5}, odds)
}

func (s *SlicesTestSuite) TestFind() {
	nums := []int{1, 2, 3, 4}
	val, ok := Find(nums, func(n int) bool { return n > 2 })
	s.True(ok)
	s.Equal(3, val)

	_, ok = Find(nums, func(n int) bool { return n > 10 })
	s.False(ok)
}

func (s *SlicesTestSuite) TestFindIndex() {
	nums := []int{1, 2, 3, 4}
	idx := FindIndex(nums, func(n int) bool { return n > 2 })
	s.Equal(2, idx)

	idx = FindIndex(nums, func(n int) bool { return n > 10 })
	s.Equal(-1, idx)
}

func (s *SlicesTestSuite) TestAny() {
	nums := []int{1, 2, 3}
	s.True(Any(nums, func(n int) bool { return n%2 == 0 }))
	s.False(Any(nums, func(n int) bool { return n > 10 }))
}

func (s *SlicesTestSuite) TestAll() {
	nums := []int{2, 4, 6}
	s.True(All(nums, func(n int) bool { return n%2 == 0 }))
	s.False(All(nums, func(n int) bool { return n > 3 }))
}

func (s *SlicesTestSuite) TestNone() {
	nums := []int{1, 3, 5}
	s.True(None(nums, func(n int) bool { return n%2 == 0 }))
	s.False(None(nums, func(n int) bool { return n > 2 }))
}

func (s *SlicesTestSuite) TestCount() {
	nums := []int{1, 2, 3, 4}
	count := Count(nums, func(n int) bool { return n%2 == 0 })
	s.Equal(2, count)
}

func (s *SlicesTestSuite) TestFlatten() {
	nested := [][]int{{1, 2}, {3, 4}, {5}}
	flat := Flatten(nested)
	s.Equal([]int{1, 2, 3, 4, 5}, flat)
}

func (s *SlicesTestSuite) TestZip() {
	keys := []string{"a", "b", "c"}
	vals := []int{1, 2, 3}
	pairs := Zip(keys, vals)
	s.Len(pairs, 3)
	s.Equal(Pair[string, int]{"a", 1}, pairs[0])
	s.Equal(Pair[string, int]{"b", 2}, pairs[1])
	s.Equal(Pair[string, int]{"c", 3}, pairs[2])
}

func (s *SlicesTestSuite) TestZipUneven() {
	keys := []string{"a", "b"}
	vals := []int{1, 2, 3, 4}
	pairs := Zip(keys, vals)
	s.Len(pairs, 2)
}

func (s *SlicesTestSuite) TestToMap() {
	pairs := []Pair[string, int]{{"a", 1}, {"b", 2}}
	m := ToMap(pairs)
	s.Equal(map[string]int{"a": 1, "b": 2}, m)
}

func (s *SlicesTestSuite) TestKeyBy() {
	type User struct {
		ID   int
		Name string
	}
	users := []User{{1, "a"}, {2, "b"}}
	m := KeyBy(users, func(u User) int { return u.ID })
	s.Equal(User{1, "a"}, m[1])
	s.Equal(User{2, "b"}, m[2])
}

func (s *SlicesTestSuite) TestCompact() {
	strs := []string{"a", "", "b", "", "c"}
	compact := Compact(strs)
	s.Equal([]string{"a", "b", "c"}, compact)

	nums := []int{1, 0, 2, 0, 3}
	compactNums := Compact(nums)
	s.Equal([]int{1, 2, 3}, compactNums)
}

func (s *SlicesTestSuite) TestFirst() {
	nums := []int{1, 2, 3}
	val, ok := First(nums)
	s.True(ok)
	s.Equal(1, val)

	var empty []int
	_, ok = First(empty)
	s.False(ok)
}

func (s *SlicesTestSuite) TestLast() {
	nums := []int{1, 2, 3}
	val, ok := Last(nums)
	s.True(ok)
	s.Equal(3, val)

	var empty []int
	_, ok = Last(empty)
	s.False(ok)
}

func (s *SlicesTestSuite) TestTake() {
	nums := []int{1, 2, 3, 4, 5}
	s.Equal([]int{1, 2, 3}, Take(nums, 3))
	s.Equal([]int{1, 2, 3, 4, 5}, Take(nums, 10))
	s.Nil(Take(nums, 0))
	s.Nil(Take(nums, -1))
}

func (s *SlicesTestSuite) TestSkip() {
	nums := []int{1, 2, 3, 4, 5}
	s.Equal([]int{3, 4, 5}, Skip(nums, 2))
	s.Nil(Skip(nums, 10))
	s.Equal(nums, Skip(nums, 0))
}

func (s *SlicesTestSuite) TestTakeWhile() {
	nums := []int{1, 2, 3, 4, 5}
	result := TakeWhile(nums, func(n int) bool { return n < 4 })
	s.Equal([]int{1, 2, 3}, result)
}

func (s *SlicesTestSuite) TestSkipWhile() {
	nums := []int{1, 2, 3, 4, 5}
	result := SkipWhile(nums, func(n int) bool { return n < 3 })
	s.Equal([]int{3, 4, 5}, result)
}

func (s *SlicesTestSuite) TestContains() {
	s.True(Contains([]int{1, 2, 3}, 2))
	s.False(Contains([]int{1, 2, 3}, 4))
	s.False(Contains([]int{}, 1))
	s.False(Contains[int](nil, 1))
}

func (s *SlicesTestSuite) TestContainsAll() {
	s.True(ContainsAll([]int{1, 2, 3}, []int{1, 2}))
	s.True(ContainsAll([]int{1, 2, 3}, []int{}))
	s.False(ContainsAll([]int{1, 2}, []int{1, 3}))
}

func (s *SlicesTestSuite) TestContainsAny() {
	s.True(ContainsAny([]int{1, 2, 3}, []int{4, 2}))
	s.False(ContainsAny([]int{1, 2, 3}, []int{4, 5}))
	s.False(ContainsAny([]int{1, 2, 3}, []int{}))
}

func (s *SlicesTestSuite) TestIntersectSet() {
	result := IntersectSet([]int{1, 2, 3, 2}, []int{2, 3, 4})
	s.Equal([]int{2, 3}, result)

	s.Empty(IntersectSet([]int{1, 2}, []int{3, 4}))
	s.Empty(IntersectSet[int](nil, []int{1}))
}

func (s *SlicesTestSuite) TestIntersectSetFunc() {
	eq := func(a, b int) bool { return a == b }
	result := IntersectSetFunc([]int{1, 2, 3}, []int{2, 3, 4}, eq)
	s.Equal([]int{2, 3}, result)
}

func (s *SlicesTestSuite) TestUnionSet() {
	result := UnionSet([]int{1, 2, 3}, []int{2, 3, 4})
	s.Equal([]int{1, 2, 3, 4}, result)
}

func (s *SlicesTestSuite) TestDiffSet() {
	result := DiffSet([]int{1, 2, 3}, []int{2, 3, 4})
	s.Equal([]int{1}, result)

	s.Empty(DiffSet([]int{1, 2}, []int{1, 2, 3}))
}

func (s *SlicesTestSuite) TestSymmetricDiffSet() {
	result := SymmetricDiffSet([]int{1, 2, 3}, []int{2, 3, 4})
	s.Equal([]int{1, 4}, result)
}

func (s *SlicesTestSuite) TestSum() {
	s.Equal(6, Sum([]int{1, 2, 3}))
	s.Equal(0, Sum[int](nil))
	s.InDelta(6.0, Sum([]float64{1.0, 2.0, 3.0}), 1e-9)
}

func (s *SlicesTestSuite) TestMin() {
	val, ok := Min([]int{3, 1, 2})
	s.True(ok)
	s.Equal(1, val)

	_, ok = Min[int](nil)
	s.False(ok)
}

func (s *SlicesTestSuite) TestMax() {
	val, ok := Max([]int{3, 1, 2})
	s.True(ok)
	s.Equal(3, val)

	_, ok = Max[int](nil)
	s.False(ok)
}

func (s *SlicesTestSuite) TestIndexAll() {
	s.Equal([]int{0, 2}, IndexAll([]int{1, 2, 1, 3}, 1))
	s.Empty(IndexAll([]int{1, 2, 3}, 4))
	s.Empty(IndexAll[int](nil, 1))
}

func (s *SlicesTestSuite) TestLastIndex() {
	s.Equal(2, LastIndex([]int{1, 2, 1, 3}, 1))
	s.Equal(-1, LastIndex([]int{1, 2, 3}, 4))
	s.Equal(-1, LastIndex[int](nil, 1))
}

func (s *SlicesTestSuite) TestReverse() {
	result := Reverse([]int{1, 2, 3})
	s.Equal([]int{3, 2, 1}, result)
	s.Empty(Reverse[int](nil))
}

func (s *SlicesTestSuite) TestReverseSelf() {
	nums := []int{1, 2, 3, 4}
	ReverseSelf(nums)
	s.Equal([]int{4, 3, 2, 1}, nums)
}

func (s *SlicesTestSuite) TestInsert() {
	result, err := Insert([]int{1, 2, 3}, 1, 10)
	s.NoError(err)
	s.Equal([]int{1, 10, 2, 3}, result)

	result, err = Insert([]int{1, 2, 3}, 0, 0)
	s.NoError(err)
	s.Equal([]int{0, 1, 2, 3}, result)

	result, err = Insert([]int{1, 2, 3}, 3, 4)
	s.NoError(err)
	s.Equal([]int{1, 2, 3, 4}, result)

	_, err = Insert([]int{1, 2, 3}, 4, 10)
	s.Error(err)

	_, err = Insert([]int{1, 2, 3}, -1, 10)
	s.Error(err)
}

func (s *SlicesTestSuite) TestDelete() {
	result, err := Delete([]int{1, 2, 3}, 1)
	s.NoError(err)
	s.Equal([]int{1, 3}, result)

	result, err = Delete([]int{1, 2, 3}, 0)
	s.NoError(err)
	s.Equal([]int{2, 3}, result)

	_, err = Delete([]int{1, 2, 3}, 3)
	s.Error(err)

	_, err = Delete([]int{1, 2, 3}, -1)
	s.Error(err)
}

func (s *SlicesTestSuite) TestIterAll() {
	slice := []string{"a", "b", "c"}
	var indices []int
	var vals []string
	for i, v := range IterAll(slice) {
		indices = append(indices, i)
		vals = append(vals, v)
	}
	s.Equal([]int{0, 1, 2}, indices)
	s.Equal([]string{"a", "b", "c"}, vals)
}

func (s *SlicesTestSuite) TestIterAllEmpty() {
	count := 0
	for range IterAll([]int{}) {
		count++
	}
	s.Equal(0, count)
}

func (s *SlicesTestSuite) TestIterValues() {
	slice := []int{10, 20, 30}
	var vals []int
	for v := range IterValues(slice) {
		vals = append(vals, v)
	}
	s.Equal([]int{10, 20, 30}, vals)
}

func (s *SlicesTestSuite) TestIterValuesEarlyBreak() {
	slice := []int{1, 2, 3, 4, 5}
	var vals []int
	for v := range IterValues(slice) {
		vals = append(vals, v)
		if v == 3 {
			break
		}
	}
	s.Equal([]int{1, 2, 3}, vals)
}

func (s *SlicesTestSuite) TestIterBackward() {
	slice := []string{"a", "b", "c"}
	var indices []int
	var vals []string
	for i, v := range IterBackward(slice) {
		indices = append(indices, i)
		vals = append(vals, v)
	}
	s.Equal([]int{2, 1, 0}, indices)
	s.Equal([]string{"c", "b", "a"}, vals)
}

func (s *SlicesTestSuite) TestIterBackwardEmpty() {
	count := 0
	for range IterBackward([]int{}) {
		count++
	}
	s.Equal(0, count)
}

func (s *SlicesTestSuite) TestCollect() {
	seq := IterValues([]int{1, 2, 3})
	result := Collect(seq)
	s.Equal([]int{1, 2, 3}, result)
}

func (s *SlicesTestSuite) TestCollectEmpty() {
	seq := IterValues([]int{})
	result := Collect(seq)
	s.Nil(result)
}
