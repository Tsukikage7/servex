package idempotency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/cache"
	"github.com/Tsukikage7/servex/v2/testx"
)

// newTestStore 创建测试用的存储.
func newTestStore() (*IdempotentStore, cache.Cache) {
	memCache, _ := cache.NewMemoryCache(nil, testx.NopLogger())
	kv := CacheKV(memCache)
	return NewStore(kv), memCache
}

func TestRedisStore(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	ctx := t.Context()
	key := "test-key"

	t.Run("get non-existent", func(t *testing.T) {
		result, err := store.Get(ctx, key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result")
		}
	})

	t.Run("set and get", func(t *testing.T) {
		result := &Result{
			StatusCode: 200,
			Body:       []byte("test body"),
			CreatedAt:  time.Now(),
		}

		err := store.Set(ctx, key, result, time.Hour)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, err := store.Get(ctx, key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected result")
		}
		if got.StatusCode != 200 {
			t.Errorf("expected 200, got %d", got.StatusCode)
		}
	})

	t.Run("setNX", func(t *testing.T) {
		newKey := "new-key"

		// First call should succeed
		ok, err := store.SetNX(ctx, newKey, time.Hour)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !ok {
			t.Error("expected true")
		}

		// Second call should fail (key locked)
		ok, err = store.SetNX(ctx, newKey, time.Hour)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected false")
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.Delete(ctx, key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		result, err := store.Get(ctx, key)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil after delete")
		}
	})
}

func TestResultEncodeDecode(t *testing.T) {
	original := &Result{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id":123}`),
		CreatedAt:  time.Now(),
	}

	data, err := original.Encode()
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	decoded, err := DecodeResult(data)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if decoded.StatusCode != original.StatusCode {
		t.Errorf("status code mismatch: %d != %d", decoded.StatusCode, original.StatusCode)
	}
	if string(decoded.Body) != string(original.Body) {
		t.Errorf("body mismatch: %s != %s", decoded.Body, original.Body)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	wrapped := HTTPMiddleware(store)(handler)

	t.Run("without key", func(t *testing.T) {
		callCount = 0
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("with key - first call", func(t *testing.T) {
		callCount = 0
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Idempotency-Key", "test-123")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("with key - second call (cached)", func(t *testing.T) {
		callCount = 0
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Idempotency-Key", "test-123")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if callCount != 0 {
			t.Errorf("expected 0 calls (cached), got %d", callCount)
		}
	})

	t.Run("GET method - no idempotency check", func(t *testing.T) {
		callCount = 0
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Idempotency-Key", "test-get")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("panic on nil store", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		HTTPMiddleware(nil)
	})
}

func TestEndpointMiddleware(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	callCount := 0
	endpoint := func(ctx context.Context, request any) (any, error) {
		callCount++
		return "result", nil
	}

	wrapped := EndpointMiddleware(store,
		WithKeyExtractor(func(ctx any) string {
			if req, ok := ctx.(map[string]string); ok {
				return req["key"]
			}
			return ""
		}),
	)(endpoint)

	t.Run("without key", func(t *testing.T) {
		callCount = 0
		resp, err := wrapped(t.Context(), map[string]string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "result" {
			t.Errorf("unexpected response: %v", resp)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("with key - first call", func(t *testing.T) {
		callCount = 0
		resp, err := wrapped(t.Context(), map[string]string{"key": "endpoint-123"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "result" {
			t.Errorf("unexpected response: %v", resp)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("with key - second call (cached)", func(t *testing.T) {
		callCount = 0
		_, err := wrapped(t.Context(), map[string]string{"key": "endpoint-123"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if callCount != 0 {
			t.Errorf("expected 0 calls (cached), got %d", callCount)
		}
	})

	t.Run("panic on nil store", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		EndpointMiddleware(nil)
	})
}

func TestIdempotentRequestInterface(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	callCount := 0
	endpoint := func(ctx context.Context, request any) (any, error) {
		callCount++
		return "result", nil
	}

	wrapped := EndpointMiddleware(store)(endpoint)

	t.Run("with IdempotentRequest", func(t *testing.T) {
		callCount = 0
		req := &testIdempotentRequest{key: "idem-req-123"}

		_, err := wrapped(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}

		// Second call should be cached
		callCount = 0
		_, err = wrapped(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if callCount != 0 {
			t.Errorf("expected 0 calls (cached), got %d", callCount)
		}
	})
}

type testIdempotentRequest struct {
	key string
}

func (r *testIdempotentRequest) IdempotencyKey() string {
	return r.key
}

func TestOptions(t *testing.T) {
	store, memCache := newTestStore()
	defer memCache.Close()

	t.Run("WithTTL", func(t *testing.T) {
		o := applyOptions(store, []Option{WithTTL(time.Hour)})
		if o.ttl != time.Hour {
			t.Errorf("expected 1h, got %v", o.ttl)
		}
	})

	t.Run("WithSkipOnError", func(t *testing.T) {
		o := applyOptions(store, []Option{WithSkipOnError(true)})
		if !o.skipOnError {
			t.Error("expected skipOnError to be true")
		}
	})

	t.Run("WithLockTimeout", func(t *testing.T) {
		o := applyOptions(store, []Option{WithLockTimeout(time.Minute)})
		if o.lockTimeout != time.Minute {
			t.Errorf("expected 1m, got %v", o.lockTimeout)
		}
	})
}
