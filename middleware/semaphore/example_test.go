package semaphore_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/middleware/semaphore"
)

// mockCounter implements semaphore.Counter for demonstration.
type mockCounter struct {
	counts map[string]int64
}

func (m *mockCounter) Increment(_ context.Context, key string) (int64, error) {
	if m.counts == nil {
		m.counts = make(map[string]int64)
	}
	m.counts[key]++
	return m.counts[key], nil
}

func (m *mockCounter) Decrement(_ context.Context, key string) (int64, error) {
	if m.counts == nil {
		m.counts = make(map[string]int64)
	}
	m.counts[key]--
	return m.counts[key], nil
}

func (m *mockCounter) Get(_ context.Context, key string) (int64, error) {
	if m.counts == nil {
		return 0, nil
	}
	return m.counts[key], nil
}

func (m *mockCounter) Expire(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func ExampleNew() {
	// Create a distributed semaphore with size 10.
	counter := &mockCounter{}
	sem := semaphore.New(counter, "api-limit", 10)

	fmt.Println("size:", sem.Size())
	// Output:
	// size: 10
}

func ExampleDistributed_TryAcquire() {
	counter := &mockCounter{}
	sem := semaphore.New(counter, "api-limit", 2)

	ctx := context.Background()
	fmt.Println("acquire 1:", sem.TryAcquire(ctx))
	fmt.Println("acquire 2:", sem.TryAcquire(ctx))
	fmt.Println("acquire 3:", sem.TryAcquire(ctx)) // exceeds size

	_ = sem.Release(ctx) // release one
	fmt.Println("acquire 4:", sem.TryAcquire(ctx))
	// Output:
	// acquire 1: true
	// acquire 2: true
	// acquire 3: false
	// acquire 4: true
}
