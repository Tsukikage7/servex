package tenant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// tokenHashKey 计算 token 的 hash 缓存 key.
func tokenHashKey(prefix, token string) string {
	h := sha256.Sum256([]byte(token))
	return prefix + hex.EncodeToString(h[:])
}

// mockCacheStore 模拟缓存存储.
type mockCacheStore struct {
	data map[string]string
}

func newMockCacheStore() *mockCacheStore {
	return &mockCacheStore{data: make(map[string]string)}
}

func (m *mockCacheStore) Get(_ context.Context, key string) (string, error) {
	val, ok := m.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	return val, nil
}

func (m *mockCacheStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	m.data[key] = value
	return nil
}

func TestCachedResolver_CacheMiss(t *testing.T) {
	inner := &mockResolver{
		tenant: &testTenant{id: "t1", enabled: true},
	}
	store := newMockCacheStore()

	cached := NewCachedResolver(inner, store,
		WithMarshal(func(t Tenant) (string, error) { return t.TenantID(), nil }),
		WithUnmarshal(func(s string) (Tenant, error) { return &testTenant{id: s, enabled: true}, nil }),
	)

	tn, err := cached.Resolve(t.Context(), "token-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tn.TenantID() != "t1" {
		t.Fatalf("TenantID = %q, want %q", tn.TenantID(), "t1")
	}

	// 验证缓存写入（使用 hash key）
	if _, ok := store.data[tokenHashKey("tenant:", "token-1")]; !ok {
		t.Fatal("应写入缓存")
	}
}

func TestCachedResolver_CacheHit(t *testing.T) {
	inner := &mockResolver{
		err: errors.New("should not be called"),
	}
	store := newMockCacheStore()
	store.data[tokenHashKey("tenant:", "token-1")] = "cached-t"

	cached := NewCachedResolver(inner, store,
		WithMarshal(func(t Tenant) (string, error) { return t.TenantID(), nil }),
		WithUnmarshal(func(s string) (Tenant, error) { return &testTenant{id: s, enabled: true}, nil }),
	)

	tn, err := cached.Resolve(t.Context(), "token-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tn.TenantID() != "cached-t" {
		t.Fatalf("TenantID = %q, want %q", tn.TenantID(), "cached-t")
	}
}

func TestCachedResolver_CustomPrefix(t *testing.T) {
	inner := &mockResolver{
		tenant: &testTenant{id: "t1", enabled: true},
	}
	store := newMockCacheStore()

	cached := NewCachedResolver(inner, store,
		WithCachePrefix("mytenant:"),
		WithMarshal(func(t Tenant) (string, error) { return t.TenantID(), nil }),
		WithUnmarshal(func(s string) (Tenant, error) { return &testTenant{id: s, enabled: true}, nil }),
	)

	_, err := cached.Resolve(t.Context(), "token-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.data[tokenHashKey("mytenant:", "token-1")]; !ok {
		t.Fatal("应使用自定义前缀写入缓存")
	}
}

func TestCachedResolver_InnerError(t *testing.T) {
	inner := &mockResolver{err: ErrTenantNotFound}
	store := newMockCacheStore()

	cached := NewCachedResolver(inner, store,
		WithMarshal(func(t Tenant) (string, error) { return t.TenantID(), nil }),
		WithUnmarshal(func(s string) (Tenant, error) { return &testTenant{id: s, enabled: true}, nil }),
	)

	_, err := cached.Resolve(t.Context(), "token-bad")
	if !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("err = %v, want ErrTenantNotFound", err)
	}
}

func TestNewCachedResolver_PanicOnNilInner(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应在 nil inner 时 panic")
		}
	}()
	NewCachedResolver(nil, newMockCacheStore(),
		WithMarshal(func(t Tenant) (string, error) { return "", nil }),
		WithUnmarshal(func(s string) (Tenant, error) { return nil, nil }),
	)
}

func TestNewCachedResolver_PanicOnNilStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应在 nil store 时 panic")
		}
	}()
	NewCachedResolver(&mockResolver{}, nil,
		WithMarshal(func(t Tenant) (string, error) { return "", nil }),
		WithUnmarshal(func(s string) (Tenant, error) { return nil, nil }),
	)
}

func TestNewCachedResolver_PanicOnNoMarshal(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应在缺少 Marshal/Unmarshal 时 panic")
		}
	}()
	NewCachedResolver(&mockResolver{}, newMockCacheStore())
}
