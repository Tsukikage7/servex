package sagakvstore

import (
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/domain/saga"
	"github.com/Tsukikage7/servex/storage/cache"
	"github.com/Tsukikage7/servex/testx"
)

func newTestStore() (*Store, cache.Cache) {
	memCache, _ := cache.NewMemoryCache(nil, testx.NopLogger())
	kv := CacheKV(memCache)
	return NewStore(kv), memCache
}

func TestStore_SaveGetDelete(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	ctx := t.Context()

	// Save
	state := saga.NewState("test-1", "test-saga", 2)
	state.Status = saga.SagaStatusCompleted

	err := store.Save(ctx, state)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Get
	got, err := store.Get(ctx, "test-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got.Name != "test-saga" {
		t.Errorf("expected name test-saga, got %s", got.Name)
	}

	// Delete
	err = store.Delete(ctx, "test-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Get after delete
	_, err = store.Get(ctx, "test-1")
	if !errors.Is(err, saga.ErrSagaNotFound) {
		t.Errorf("expected ErrSagaNotFound, got %v", err)
	}
}

func TestStore_List(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	// List should return nil for KV store
	result, err := store.List(t.Context(), saga.SagaStatusCompleted, 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}
