package priorityqueue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConcurrentPQTestSuite struct {
	suite.Suite
}

func TestConcurrentPQSuite(t *testing.T) {
	suite.Run(t, new(ConcurrentPQTestSuite))
}

func (s *ConcurrentPQTestSuite) TestNewConcurrentMin() {
	cpq := NewConcurrentMin[int]()
	s.NotNil(cpq)
	s.True(cpq.IsEmpty())
	s.Equal(0, cpq.Len())
}

func (s *ConcurrentPQTestSuite) TestNewConcurrentMax() {
	cpq := NewConcurrentMax[int]()
	cpq.Push(1, 3, 2)

	val, ok := cpq.Pop()
	s.True(ok)
	s.Equal(3, val)
}

func (s *ConcurrentPQTestSuite) TestNewConcurrent_Custom() {
	type task struct {
		name     string
		priority int
	}

	cpq := NewConcurrent(func(a, b task) bool {
		return a.priority > b.priority
	})

	cpq.Push(task{"low", 1}, task{"high", 10}, task{"mid", 5})

	val, ok := cpq.Pop()
	s.True(ok)
	s.Equal("high", val.name)
}

func (s *ConcurrentPQTestSuite) TestPushPop() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(5, 3, 7, 1)

	s.Equal(4, cpq.Len())

	val, ok := cpq.Pop()
	s.True(ok)
	s.Equal(1, val)

	val, ok = cpq.Pop()
	s.True(ok)
	s.Equal(3, val)
}

func (s *ConcurrentPQTestSuite) TestPopEmpty() {
	cpq := NewConcurrentMin[int]()
	val, ok := cpq.Pop()
	s.False(ok)
	s.Equal(0, val)
}

func (s *ConcurrentPQTestSuite) TestPeek() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(3, 1, 2)

	val, ok := cpq.Peek()
	s.True(ok)
	s.Equal(1, val)
	s.Equal(3, cpq.Len())
}

func (s *ConcurrentPQTestSuite) TestPeekEmpty() {
	cpq := NewConcurrentMin[int]()
	val, ok := cpq.Peek()
	s.False(ok)
	s.Equal(0, val)
}

func (s *ConcurrentPQTestSuite) TestClear() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(1, 2, 3)
	s.Equal(3, cpq.Len())

	cpq.Clear()
	s.True(cpq.IsEmpty())
	s.Equal(0, cpq.Len())
}

func (s *ConcurrentPQTestSuite) TestDrainToSlice() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(3, 1, 2)

	slice := cpq.DrainToSlice()
	s.Equal([]int{1, 2, 3}, slice)
	s.True(cpq.IsEmpty())
}

func (s *ConcurrentPQTestSuite) TestClone() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(3, 1, 2)

	clone := cpq.Clone()
	s.Equal(cpq.Len(), clone.Len())

	clone.Pop()
	s.Equal(3, cpq.Len())
	s.Equal(2, clone.Len())
}

func (s *ConcurrentPQTestSuite) TestConcurrentPushPop() {
	cpq := NewConcurrentMin[int]()
	const numGoroutines = 100
	const numOps = 100

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			for j := range numOps {
				cpq.Push(i*numOps + j)
			}
		})
	}
	wg.Wait()

	s.Equal(numGoroutines*numOps, cpq.Len())

	results := make([]int, 0, numGoroutines*numOps)
	var mu sync.Mutex

	for range numGoroutines {
		wg.Go(func() {
			for range numOps {
				if val, ok := cpq.Pop(); ok {
					mu.Lock()
					results = append(results, val)
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()

	s.Equal(numGoroutines*numOps, len(results))
	s.True(cpq.IsEmpty())
}

func (s *ConcurrentPQTestSuite) TestConcurrentMixedOps() {
	cpq := NewConcurrentMin[int]()

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Go(func() {
			cpq.Push(i)
		})

		wg.Go(func() {
			cpq.Peek()
		})

		wg.Go(func() {
			cpq.Len()
		})
	}
	wg.Wait()
}

func (s *ConcurrentPQTestSuite) TestConcurrentClone() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(5, 3, 1, 4, 2)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			clone := cpq.Clone()
			s.Equal(5, clone.Len())
		})
	}
	wg.Wait()
}

func (s *ConcurrentPQTestSuite) TestAll() {
	cpq := NewConcurrentMin[int]()
	cpq.Push(3, 1, 2)

	var items []int
	for item := range cpq.All() {
		items = append(items, item)
	}
	s.Equal([]int{1, 2, 3}, items)
	// 原队列不受影响
	s.Equal(3, cpq.Len())
}

func (s *ConcurrentPQTestSuite) TestAllEmpty() {
	cpq := NewConcurrentMin[int]()
	count := 0
	for range cpq.All() {
		count++
	}
	s.Equal(0, count)
}
