package mapsx

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MapsTestSuite struct {
	suite.Suite
}

func TestMapsSuite(t *testing.T) {
	suite.Run(t, new(MapsTestSuite))
}

func (s *MapsTestSuite) TestKeys() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := Keys(m)
	s.Len(keys, 3)
	s.ElementsMatch([]string{"a", "b", "c"}, keys)
}

func (s *MapsTestSuite) TestKeysEmpty() {
	m := map[string]int{}
	keys := Keys(m)
	s.Empty(keys)
}

func (s *MapsTestSuite) TestValues() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	values := Values(m)
	s.Len(values, 3)
	s.ElementsMatch([]int{1, 2, 3}, values)
}

func (s *MapsTestSuite) TestEntries() {
	m := map[string]int{"a": 1, "b": 2}
	entries := Entries(m)
	s.Len(entries, 2)
}

func (s *MapsTestSuite) TestFromEntries() {
	entries := []Entry[string, int]{{"a", 1}, {"b", 2}}
	m := FromEntries(entries)
	s.Equal(map[string]int{"a": 1, "b": 2}, m)
}

func (s *MapsTestSuite) TestMerge() {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"b": 20, "c": 3}
	m3 := map[string]int{"c": 30, "d": 4}

	merged := Merge(m1, m2, m3)
	s.Equal(map[string]int{"a": 1, "b": 20, "c": 30, "d": 4}, merged)
}

func (s *MapsTestSuite) TestMergeEmpty() {
	merged := Merge[string, int]()
	s.Empty(merged)
}

func (s *MapsTestSuite) TestFilter() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	filtered := Filter(m, func(k string, v int) bool { return v > 1 })
	s.Equal(map[string]int{"b": 2, "c": 3}, filtered)
}

func (s *MapsTestSuite) TestFilterKeys() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	filtered := FilterKeys(m, "a", "c")
	s.Equal(map[string]int{"a": 1, "c": 3}, filtered)
}

func (s *MapsTestSuite) TestFilterKeysNonExistent() {
	m := map[string]int{"a": 1, "b": 2}
	filtered := FilterKeys(m, "a", "x")
	s.Equal(map[string]int{"a": 1}, filtered)
}

func (s *MapsTestSuite) TestOmitKeys() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	filtered := OmitKeys(m, "a", "c")
	s.Equal(map[string]int{"b": 2}, filtered)
}

func (s *MapsTestSuite) TestMapKeys() {
	m := map[int]string{1: "a", 2: "b"}
	mapped := MapKeys(m, strconv.Itoa)
	s.Equal(map[string]string{"1": "a", "2": "b"}, mapped)
}

func (s *MapsTestSuite) TestMapValues() {
	m := map[string]int{"a": 1, "b": 2}
	mapped := MapValues(m, func(v int) int { return v * 10 })
	s.Equal(map[string]int{"a": 10, "b": 20}, mapped)
}

func (s *MapsTestSuite) TestInvert() {
	m := map[string]int{"a": 1, "b": 2}
	inverted := Invert(m)
	s.Equal(map[int]string{1: "a", 2: "b"}, inverted)
}

func (s *MapsTestSuite) TestClone() {
	m := map[string]int{"a": 1, "b": 2}
	cloned := Clone(m)
	s.Equal(m, cloned)

	// 修改 clone 不影响原 map
	cloned["c"] = 3
	s.NotContains(m, "c")
}

func (s *MapsTestSuite) TestEqual() {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"a": 1, "b": 2}
	m3 := map[string]int{"a": 1, "b": 3}
	m4 := map[string]int{"a": 1}

	s.True(Equal(m1, m2))
	s.False(Equal(m1, m3))
	s.False(Equal(m1, m4))
}

func (s *MapsTestSuite) TestEqualBy() {
	m1 := map[string][]int{"a": {1, 2}}
	m2 := map[string][]int{"a": {1, 2}}

	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	s.True(EqualBy(m1, m2, eq))
}

func (s *MapsTestSuite) TestGetOrDefault() {
	m := map[string]int{"a": 1}
	s.Equal(1, GetOrDefault(m, "a", 100))
	s.Equal(100, GetOrDefault(m, "b", 100))
}

func (s *MapsTestSuite) TestGetOrPut() {
	m := map[string]int{"a": 1}
	s.Equal(1, GetOrPut(m, "a", 100))
	s.Equal(100, GetOrPut(m, "b", 100))
	s.Equal(100, m["b"]) // 已被添加
}

func (s *MapsTestSuite) TestGetOrCompute() {
	m := map[string]int{"a": 1}
	callCount := 0

	// 存在时不调用 compute
	val := GetOrCompute(m, "a", func() int {
		callCount++
		return 100
	})
	s.Equal(1, val)
	s.Equal(0, callCount)

	// 不存在时调用 compute
	val = GetOrCompute(m, "b", func() int {
		callCount++
		return 100
	})
	s.Equal(100, val)
	s.Equal(1, callCount)
	s.Equal(100, m["b"])
}

func (s *MapsTestSuite) TestForEach() {
	m := map[string]int{"a": 1, "b": 2}
	sum := 0
	ForEach(m, func(k string, v int) {
		sum += v
	})
	s.Equal(3, sum)
}

func (s *MapsTestSuite) TestAny() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	s.True(Any(m, func(k string, v int) bool { return v > 2 }))
	s.False(Any(m, func(k string, v int) bool { return v > 10 }))
}

func (s *MapsTestSuite) TestAll() {
	m := map[string]int{"a": 2, "b": 4, "c": 6}
	s.True(All(m, func(k string, v int) bool { return v%2 == 0 }))
	s.False(All(m, func(k string, v int) bool { return v > 3 }))
}

func (s *MapsTestSuite) TestNone() {
	m := map[string]int{"a": 1, "b": 3, "c": 5}
	s.True(None(m, func(k string, v int) bool { return v%2 == 0 }))
	s.False(None(m, func(k string, v int) bool { return v > 2 }))
}

func (s *MapsTestSuite) TestCount() {
	m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}
	count := Count(m, func(k string, v int) bool { return v%2 == 0 })
	s.Equal(2, count)
}

func (s *MapsTestSuite) TestContainsKey() {
	m := map[string]int{"a": 1, "b": 2}
	s.True(ContainsKey(m, "a"))
	s.False(ContainsKey(m, "c"))
}

func (s *MapsTestSuite) TestContainsValue() {
	m := map[string]int{"a": 1, "b": 2}
	s.True(ContainsValue(m, 1))
	s.False(ContainsValue(m, 3))
}

func (s *MapsTestSuite) TestFindKey() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	key, ok := FindKey(m, func(k string, v int) bool { return v > 2 })
	s.True(ok)
	s.Equal("c", key)

	_, ok = FindKey(m, func(k string, v int) bool { return v > 10 })
	s.False(ok)
}

func (s *MapsTestSuite) TestDiff() {
	m1 := map[string]int{"a": 1, "b": 2, "c": 3}
	m2 := map[string]int{"b": 2, "c": 30, "d": 4}

	added, removed, changed := Diff(m1, m2)

	s.ElementsMatch([]string{"d"}, added)
	s.ElementsMatch([]string{"a"}, removed)
	s.ElementsMatch([]string{"c"}, changed)
}

func (s *MapsTestSuite) TestDiffIdentical() {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"a": 1, "b": 2}

	added, removed, changed := Diff(m1, m2)

	s.Empty(added)
	s.Empty(removed)
	s.Empty(changed)
}

func (s *MapsTestSuite) TestKeysValues() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys, values := KeysValues(m)
	s.Len(keys, 3)
	s.Len(values, 3)
	// 验证 keys[i] 与 values[i] 对应
	for i, k := range keys {
		s.Equal(m[k], values[i])
	}
}

func (s *MapsTestSuite) TestKeysValuesEmpty() {
	keys, values := KeysValues(map[string]int{})
	s.Empty(keys)
	s.Empty(values)
}

func (s *MapsTestSuite) TestMergeFunc() {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"a": 10, "c": 3}
	result := MergeFunc(func(v1, v2 int) int { return v1 + v2 }, m1, m2)
	s.Equal(11, result["a"]) // 1 + 10
	s.Equal(2, result["b"])
	s.Equal(3, result["c"])
}

func (s *MapsTestSuite) TestMergeFunc_LastWins() {
	m1 := map[string]int{"a": 1}
	m2 := map[string]int{"a": 2}
	result := MergeFunc(func(_, v2 int) int { return v2 }, m1, m2)
	s.Equal(2, result["a"])
}

func (s *MapsTestSuite) TestIterAll() {
	m := map[string]int{"a": 1, "b": 2}
	collected := make(map[string]int)
	for k, v := range IterAll(m) {
		collected[k] = v
	}
	s.Equal(m, collected)
}

func (s *MapsTestSuite) TestIterAllEmpty() {
	count := 0
	for range IterAll(map[string]int{}) {
		count++
	}
	s.Equal(0, count)
}

func (s *MapsTestSuite) TestIterKeys() {
	m := map[string]int{"a": 1, "b": 2}
	keys := make(map[string]bool)
	for k := range IterKeys(m) {
		keys[k] = true
	}
	s.True(keys["a"])
	s.True(keys["b"])
	s.Len(keys, 2)
}

func (s *MapsTestSuite) TestIterValues() {
	m := map[string]int{"a": 1, "b": 2}
	vals := make(map[int]bool)
	for v := range IterValues(m) {
		vals[v] = true
	}
	s.True(vals[1])
	s.True(vals[2])
	s.Len(vals, 2)
}

func (s *MapsTestSuite) TestIterAllEarlyBreak() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	count := 0
	for range IterAll(m) {
		count++
		if count == 1 {
			break
		}
	}
	s.Equal(1, count)
}

func (s *MapsTestSuite) TestCollect() {
	m := map[string]int{"a": 1, "b": 2}
	seq := IterAll(m)
	result := Collect(seq)
	s.Equal(m, result)
}

func (s *MapsTestSuite) TestCollectEmpty() {
	seq := IterAll(map[string]int{})
	result := Collect(seq)
	s.Empty(result)
}
