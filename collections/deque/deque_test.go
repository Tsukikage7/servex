package deque

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type DequeTestSuite struct {
	suite.Suite
}

func TestDequeSuite(t *testing.T) {
	suite.Run(t, new(DequeTestSuite))
}

func (s *DequeTestSuite) TestNew() {
	dq := New[int]()
	s.NotNil(dq)
	s.True(dq.IsEmpty())
	s.Equal(0, dq.Len())
}

func (s *DequeTestSuite) TestNewWithCapacity() {
	dq := NewWithCapacity[int](100)
	s.NotNil(dq)
	s.True(dq.IsEmpty())
}

func (s *DequeTestSuite) TestFrom() {
	dq := From([]int{1, 2, 3})
	s.Equal(3, dq.Len())
	s.Equal([]int{1, 2, 3}, dq.ToSlice())
}

func (s *DequeTestSuite) TestPushFront() {
	dq := New[int]()
	dq.PushFront(3)
	dq.PushFront(2)
	dq.PushFront(1)

	s.Equal(3, dq.Len())
	s.Equal([]int{1, 2, 3}, dq.ToSlice())
}

func (s *DequeTestSuite) TestPushBack() {
	dq := New[int]()
	dq.PushBack(1)
	dq.PushBack(2)
	dq.PushBack(3)

	s.Equal(3, dq.Len())
	s.Equal([]int{1, 2, 3}, dq.ToSlice())
}

func (s *DequeTestSuite) TestPopFront() {
	dq := From([]int{1, 2, 3})

	val, ok := dq.PopFront()
	s.True(ok)
	s.Equal(1, val)
	s.Equal(2, dq.Len())

	val, ok = dq.PopFront()
	s.True(ok)
	s.Equal(2, val)

	val, ok = dq.PopFront()
	s.True(ok)
	s.Equal(3, val)

	_, ok = dq.PopFront()
	s.False(ok)
}

func (s *DequeTestSuite) TestPopBack() {
	dq := From([]int{1, 2, 3})

	val, ok := dq.PopBack()
	s.True(ok)
	s.Equal(3, val)
	s.Equal(2, dq.Len())

	val, ok = dq.PopBack()
	s.True(ok)
	s.Equal(2, val)

	val, ok = dq.PopBack()
	s.True(ok)
	s.Equal(1, val)

	_, ok = dq.PopBack()
	s.False(ok)
}

func (s *DequeTestSuite) TestPeekFront() {
	dq := New[int]()

	_, ok := dq.PeekFront()
	s.False(ok)

	dq.PushBack(1)
	dq.PushBack(2)

	val, ok := dq.PeekFront()
	s.True(ok)
	s.Equal(1, val)
	s.Equal(2, dq.Len()) // Peek 不应该移除元素
}

func (s *DequeTestSuite) TestPeekBack() {
	dq := New[int]()

	_, ok := dq.PeekBack()
	s.False(ok)

	dq.PushBack(1)
	dq.PushBack(2)

	val, ok := dq.PeekBack()
	s.True(ok)
	s.Equal(2, val)
	s.Equal(2, dq.Len())
}

func (s *DequeTestSuite) TestAt() {
	dq := From([]int{10, 20, 30, 40})

	val, ok := dq.At(0)
	s.True(ok)
	s.Equal(10, val)

	val, ok = dq.At(2)
	s.True(ok)
	s.Equal(30, val)

	_, ok = dq.At(-1)
	s.False(ok)

	_, ok = dq.At(4)
	s.False(ok)
}

func (s *DequeTestSuite) TestSet() {
	dq := From([]int{1, 2, 3})

	ok := dq.Set(1, 20)
	s.True(ok)

	val, _ := dq.At(1)
	s.Equal(20, val)

	ok = dq.Set(-1, 0)
	s.False(ok)

	ok = dq.Set(3, 0)
	s.False(ok)
}

func (s *DequeTestSuite) TestClear() {
	dq := From([]int{1, 2, 3})
	dq.Clear()

	s.True(dq.IsEmpty())
	s.Equal(0, dq.Len())
}

func (s *DequeTestSuite) TestClone() {
	dq := From([]int{1, 2, 3})
	clone := dq.Clone()

	s.Equal(dq.ToSlice(), clone.ToSlice())

	// 修改 clone 不影响原队列
	clone.PopFront()
	s.Equal(3, dq.Len())
	s.Equal(2, clone.Len())
}

func (s *DequeTestSuite) TestForEach() {
	dq := From([]int{1, 2, 3})
	var result []int

	dq.ForEach(func(v int) {
		result = append(result, v)
	})

	s.Equal([]int{1, 2, 3}, result)
}

func (s *DequeTestSuite) TestForEachReverse() {
	dq := From([]int{1, 2, 3})
	var result []int

	dq.ForEachReverse(func(v int) {
		result = append(result, v)
	})

	s.Equal([]int{3, 2, 1}, result)
}

func (s *DequeTestSuite) TestRotateRight() {
	dq := From([]int{1, 2, 3, 4, 5})
	dq.Rotate(2)
	s.Equal([]int{3, 4, 5, 1, 2}, dq.ToSlice())
}

func (s *DequeTestSuite) TestRotateLeft() {
	dq := From([]int{1, 2, 3, 4, 5})
	dq.Rotate(-2)
	s.Equal([]int{4, 5, 1, 2, 3}, dq.ToSlice())
}

func (s *DequeTestSuite) TestRotateZero() {
	dq := From([]int{1, 2, 3})
	dq.Rotate(0)
	s.Equal([]int{1, 2, 3}, dq.ToSlice())
}

func (s *DequeTestSuite) TestRotateEmpty() {
	dq := New[int]()
	dq.Rotate(5) // 不应该 panic
	s.True(dq.IsEmpty())
}

func (s *DequeTestSuite) TestReverse() {
	dq := From([]int{1, 2, 3, 4, 5})
	dq.Reverse()
	s.Equal([]int{5, 4, 3, 2, 1}, dq.ToSlice())
}

func (s *DequeTestSuite) TestReverseEmpty() {
	dq := New[int]()
	dq.Reverse()
	s.True(dq.IsEmpty())
}

func (s *DequeTestSuite) TestReverseSingle() {
	dq := From([]int{1})
	dq.Reverse()
	s.Equal([]int{1}, dq.ToSlice())
}

func (s *DequeTestSuite) TestGrowth() {
	dq := New[int]()

	// 添加足够多的元素触发扩容
	for i := 0; i < 100; i++ {
		dq.PushBack(i)
	}

	s.Equal(100, dq.Len())

	// 验证顺序正确
	for i := 0; i < 100; i++ {
		val, ok := dq.At(i)
		s.True(ok)
		s.Equal(i, val)
	}
}

func (s *DequeTestSuite) TestShrink() {
	dq := New[int]()

	// 添加元素
	for i := 0; i < 100; i++ {
		dq.PushBack(i)
	}

	// 移除大部分元素
	for i := 0; i < 95; i++ {
		dq.PopFront()
	}

	s.Equal(5, dq.Len())

	// 验证剩余元素正确
	expected := []int{95, 96, 97, 98, 99}
	s.Equal(expected, dq.ToSlice())
}

func (s *DequeTestSuite) TestMixedOperations() {
	dq := New[int]()

	dq.PushBack(3)
	dq.PushBack(4)
	dq.PushFront(2)
	dq.PushFront(1)
	// [1, 2, 3, 4]

	val, _ := dq.PopFront()
	s.Equal(1, val)
	// [2, 3, 4]

	val, _ = dq.PopBack()
	s.Equal(4, val)
	// [2, 3]

	dq.PushBack(5)
	// [2, 3, 5]

	s.Equal([]int{2, 3, 5}, dq.ToSlice())
}

func (s *DequeTestSuite) TestAsStack() {
	// 使用 Deque 作为栈（LIFO）
	stack := New[string]()

	stack.PushBack("a")
	stack.PushBack("b")
	stack.PushBack("c")

	val, _ := stack.PopBack()
	s.Equal("c", val)

	val, _ = stack.PopBack()
	s.Equal("b", val)

	val, _ = stack.PopBack()
	s.Equal("a", val)
}

func (s *DequeTestSuite) TestAsQueue() {
	// 使用 Deque 作为队列（FIFO）
	queue := New[string]()

	queue.PushBack("a")
	queue.PushBack("b")
	queue.PushBack("c")

	val, _ := queue.PopFront()
	s.Equal("a", val)

	val, _ = queue.PopFront()
	s.Equal("b", val)

	val, _ = queue.PopFront()
	s.Equal("c", val)
}

func (s *DequeTestSuite) TestWraparound() {
	// 测试环形缓冲区的边界情况
	dq := New[int]()

	// 填满初始容量
	for i := 0; i < 8; i++ {
		dq.PushBack(i)
	}

	// 从前面移除一些
	for i := 0; i < 4; i++ {
		dq.PopFront()
	}

	// 再添加一些（会绕到数组开头）
	for i := 8; i < 12; i++ {
		dq.PushBack(i)
	}

	expected := []int{4, 5, 6, 7, 8, 9, 10, 11}
	s.Equal(expected, dq.ToSlice())
}

func (s *DequeTestSuite) TestAll() {
	dq := From([]int{10, 20, 30})
	var indices []int
	var vals []int
	for i, v := range dq.All() {
		indices = append(indices, i)
		vals = append(vals, v)
	}
	s.Equal([]int{0, 1, 2}, indices)
	s.Equal([]int{10, 20, 30}, vals)
}

func (s *DequeTestSuite) TestAllEmpty() {
	dq := New[int]()
	count := 0
	for range dq.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *DequeTestSuite) TestAllEarlyBreak() {
	dq := From([]int{1, 2, 3, 4, 5})
	var vals []int
	for _, v := range dq.All() {
		vals = append(vals, v)
		if v == 3 {
			break
		}
	}
	s.Equal([]int{1, 2, 3}, vals)
}

func (s *DequeTestSuite) TestBackwardIter() {
	dq := From([]int{10, 20, 30})
	var indices []int
	var vals []int
	for i, v := range dq.Backward() {
		indices = append(indices, i)
		vals = append(vals, v)
	}
	s.Equal([]int{2, 1, 0}, indices)
	s.Equal([]int{30, 20, 10}, vals)
}

func (s *DequeTestSuite) TestBackwardIterEmpty() {
	dq := New[int]()
	count := 0
	for range dq.Backward() {
		count++
	}
	s.Equal(0, count)
}

func (s *DequeTestSuite) TestValues() {
	dq := From([]int{10, 20, 30})
	var vals []int
	for v := range dq.Values() {
		vals = append(vals, v)
	}
	s.Equal([]int{10, 20, 30}, vals)
}

func (s *DequeTestSuite) TestValuesEmpty() {
	dq := New[int]()
	count := 0
	for range dq.Values() {
		count++
	}
	s.Equal(0, count)
}
