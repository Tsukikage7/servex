// oauth2/state/memory_test.go
package state

import (
	"testing"
	"time"

	"github.com/Tsukikage7/servex/oauth2"
)

func TestMemoryStore_GenerateAndValidate(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := t.Context()

	state, err := s.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state == "" {
		t.Fatal("state should not be empty")
	}

	ok, err := s.Validate(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("valid state should return true")
	}

	// 第二次验证应该失败（一次性使用）
	ok, _ = s.Validate(ctx, state)
	if ok {
		t.Error("state should be consumed after first validation")
	}
}

func TestMemoryStore_InvalidState(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ok, err := s.Validate(t.Context(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("nonexistent state should return false")
	}
}

func TestMemoryStore_ImplementsInterface(t *testing.T) {
	var _ oauth2.StateStore = (*MemoryStore)(nil)
}

func TestMemoryStore_ExpiredState(t *testing.T) {
	s := NewMemoryStore(WithMemoryTTL(0))
	defer s.Close()
	ctx := t.Context()

	state, err := s.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Validate should fail because state expired.
	ok, err := s.Validate(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expired state should return false")
	}
}

func TestMemoryStore_DoubleValidate(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := t.Context()

	state, _ := s.Generate(ctx)
	ok1, _ := s.Validate(ctx, state)
	ok2, _ := s.Validate(ctx, state)

	if !ok1 {
		t.Error("first validation should succeed")
	}
	if ok2 {
		t.Error("second validation should fail (consumed)")
	}
}

func TestMemoryStore_MultipleStates(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := t.Context()

	s1, _ := s.Generate(ctx)
	s2, _ := s.Generate(ctx)

	if s1 == s2 {
		t.Error("generated states should be unique")
	}

	ok1, _ := s.Validate(ctx, s1)
	ok2, _ := s.Validate(ctx, s2)

	if !ok1 || !ok2 {
		t.Error("both states should be valid")
	}
}

func TestMemoryStore_Close(t *testing.T) {
	s := NewMemoryStore()
	// Close should return without blocking.
	s.Close()
}

func TestMemoryStore_CleanupRemovesExpired(t *testing.T) {
	s := NewMemoryStore(
		WithMemoryTTL(10*time.Millisecond),
		WithCleanupInterval(20*time.Millisecond),
	)
	defer s.Close()
	ctx := t.Context()

	state, _ := s.Generate(ctx)
	// Wait for state to expire and cleanup to run.
	time.Sleep(50 * time.Millisecond)

	// State should have been cleaned up.
	s.mu.Lock()
	_, exists := s.states[state]
	s.mu.Unlock()
	if exists {
		t.Error("expired state should have been cleaned up")
	}
}

func TestMemoryStore_WithCleanupInterval(t *testing.T) {
	s := NewMemoryStore(WithCleanupInterval(1 * time.Hour))
	defer s.Close()
	if s.cleanupInterval != 1*time.Hour {
		t.Errorf("cleanupInterval = %v, want 1h", s.cleanupInterval)
	}
}

func TestMemoryStore_CodeVerifier(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()
	ctx := t.Context()

	state, _ := s.Generate(ctx)
	s.SetCodeVerifier(state, "test-verifier")

	v := s.ConsumeCodeVerifier(state)
	if v != "test-verifier" {
		t.Errorf("code_verifier = %q, want test-verifier", v)
	}

	// Second consume should return empty.
	v2 := s.ConsumeCodeVerifier(state)
	if v2 != "" {
		t.Errorf("second consume should be empty, got %q", v2)
	}
}
