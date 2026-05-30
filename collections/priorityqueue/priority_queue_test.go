package priorityqueue

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type PriorityQueueTestSuite struct {
	suite.Suite
}

func TestPriorityQueueSuite(t *testing.T) {
	suite.Run(t, new(PriorityQueueTestSuite))
}

func (s *PriorityQueueTestSuite) TestNewMin() {
	pq := NewMin[int]()
	s.NotNil(pq)
	s.True(pq.IsEmpty())
}

func (s *PriorityQueueTestSuite) TestNewMax() {
	pq := NewMax[int]()
	s.NotNil(pq)
	s.True(pq.IsEmpty())
}

func (s *PriorityQueueTestSuite) TestMinHeap() {
	pq := NewMin[int]()
	pq.Push(3, 1, 4, 1, 5, 9, 2, 6)

	s.Equal(8, pq.Len())

	// 应该按升序弹出
	expected := []int{1, 1, 2, 3, 4, 5, 6, 9}
	for _, exp := range expected {
		val, ok := pq.Pop()
		s.True(ok)
		s.Equal(exp, val)
	}

	s.True(pq.IsEmpty())
}

func (s *PriorityQueueTestSuite) TestMaxHeap() {
	pq := NewMax[int]()
	pq.Push(3, 1, 4, 1, 5, 9, 2, 6)

	// 应该按降序弹出
	expected := []int{9, 6, 5, 4, 3, 2, 1, 1}
	for _, exp := range expected {
		val, ok := pq.Pop()
		s.True(ok)
		s.Equal(exp, val)
	}
}

func (s *PriorityQueueTestSuite) TestPeek() {
	pq := NewMin[int]()

	_, ok := pq.Peek()
	s.False(ok)

	pq.Push(3, 1, 2)

	val, ok := pq.Peek()
	s.True(ok)
	s.Equal(1, val)
	s.Equal(3, pq.Len()) // Peek 不应该移除元素
}

func (s *PriorityQueueTestSuite) TestPopEmpty() {
	pq := NewMin[int]()

	_, ok := pq.Pop()
	s.False(ok)
}

func (s *PriorityQueueTestSuite) TestClear() {
	pq := NewMin[int]()
	pq.Push(1, 2, 3)

	pq.Clear()
	s.True(pq.IsEmpty())
	s.Equal(0, pq.Len())
}

func (s *PriorityQueueTestSuite) TestDrainToSlice() {
	pq := NewMin[int]()
	pq.Push(3, 1, 2)

	slice := pq.DrainToSlice()
	s.Equal([]int{1, 2, 3}, slice)
	s.True(pq.IsEmpty())
}

func (s *PriorityQueueTestSuite) TestClone() {
	pq := NewMin[int]()
	pq.Push(3, 1, 2)

	clone := pq.Clone()
	s.Equal(3, clone.Len())

	// 修改 clone 不影响原队列
	clone.Pop()
	s.Equal(3, pq.Len())
	s.Equal(2, clone.Len())
}

func (s *PriorityQueueTestSuite) TestCustomLess() {
	type Task struct {
		Name     string
		Priority int
	}

	// 高优先级先出
	pq := New(func(a, b Task) bool {
		return a.Priority > b.Priority
	})

	pq.Push(
		Task{"low", 1},
		Task{"high", 10},
		Task{"medium", 5},
	)

	task, _ := pq.Pop()
	s.Equal("high", task.Name)

	task, _ = pq.Pop()
	s.Equal("medium", task.Name)

	task, _ = pq.Pop()
	s.Equal("low", task.Name)
}

func (s *PriorityQueueTestSuite) TestStringHeap() {
	pq := NewMin[string]()
	pq.Push("banana", "apple", "cherry")

	val, _ := pq.Pop()
	s.Equal("apple", val)

	val, _ = pq.Pop()
	s.Equal("banana", val)

	val, _ = pq.Pop()
	s.Equal("cherry", val)
}

func (s *PriorityQueueTestSuite) TestLargeDataset() {
	pq := NewMin[int]()
	n := 10000

	// 逆序插入
	for i := n; i > 0; i-- {
		pq.Push(i)
	}

	// 应该按升序弹出
	for i := 1; i <= n; i++ {
		val, ok := pq.Pop()
		s.True(ok)
		s.Equal(i, val)
	}
}

func (s *PriorityQueueTestSuite) TestAll() {
	pq := NewMin[int]()
	pq.Push(3, 1, 2)

	var items []int
	for item := range pq.All() {
		items = append(items, item)
	}
	// 按优先级顺序产出
	s.Equal([]int{1, 2, 3}, items)
	// 原队列不受影响
	s.Equal(3, pq.Len())
}

func (s *PriorityQueueTestSuite) TestAllEmpty() {
	pq := NewMin[int]()
	count := 0
	for range pq.All() {
		count++
	}
	s.Equal(0, count)
}

func (s *PriorityQueueTestSuite) TestAllEarlyBreak() {
	pq := NewMin[int]()
	pq.Push(3, 1, 2, 4, 5)

	var items []int
	for item := range pq.All() {
		items = append(items, item)
		if item == 2 {
			break
		}
	}
	s.Equal([]int{1, 2}, items)
	// 原队列不受影响
	s.Equal(5, pq.Len())
}
