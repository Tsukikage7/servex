package state

import (
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/oauth2"
)

func TestNewRedisStore_NilClient(t *testing.T) {
	_, err := NewRedisStore(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestRedisStore_ImplementsInterface(t *testing.T) {
	var _ oauth2.StateStore = (*RedisStore)(nil)
}

func TestRedisStore_WithPrefix(t *testing.T) {
	// We cannot create a real RedisStore without a cache, but we can test
	// the options by passing them. We use a nil-client test to verify
	// NewRedisStore returns error even with options.
	_, err := NewRedisStore(nil, WithPrefix("custom:"), WithTTL(5*time.Minute))
	if err == nil {
		t.Fatal("expected error for nil client even with options")
	}
}
