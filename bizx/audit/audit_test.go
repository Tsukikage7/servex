package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	store := NewMemoryStore()
	l := NewLogger(store)

	entry := &Entry{
		Actor:    "user-1",
		Action:   "create",
		Resource: "order",
		Changes: map[string]Change{
			"status": {From: nil, To: "created"},
		},
	}

	err := l.Log(t.Context(), entry)
	require.NoError(t, err)
	assert.NotEmpty(t, entry.ID)

	entries, err := l.Query(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "user-1", entries[0].Actor)
}

func TestQuery_ByActor(t *testing.T) {
	store := NewMemoryStore()
	l := NewLogger(store)

	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "create", Resource: "order"})
	_ = l.Log(t.Context(), &Entry{Actor: "bob", Action: "update", Resource: "order"})
	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "delete", Resource: "order"})

	entries, err := l.Query(t.Context(), &Filter{Actor: "alice"})
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestQuery_ByResource(t *testing.T) {
	store := NewMemoryStore()
	l := NewLogger(store)

	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "create", Resource: "order"})
	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "create", Resource: "product"})

	entries, err := l.Query(t.Context(), &Filter{Resource: "product"})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "product", entries[0].Resource)
}

func TestQuery_DateRange(t *testing.T) {
	store := NewMemoryStore()
	l := NewLogger(store)

	now := time.Now()
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "create", Resource: "order", CreatedAt: past})
	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "update", Resource: "order", CreatedAt: now})
	_ = l.Log(t.Context(), &Entry{Actor: "alice", Action: "delete", Resource: "order", CreatedAt: future})

	entries, err := l.Query(t.Context(), &Filter{
		From: now.Add(-time.Minute),
		To:   now.Add(time.Minute),
	})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "update", entries[0].Action)
}

func TestHTTPMiddleware(t *testing.T) {
	store := NewMemoryStore()
	l := NewLogger(store)

	handler := HTTPMiddleware(l,
		WithActorExtractor(func(r *http.Request) string {
			return r.Header.Get("X-User")
		}),
		WithResourceExtractor(func(r *http.Request) (string, string) {
			return "test-resource", "123"
		}),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	req.Header.Set("X-User", "alice")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	entries, err := l.Query(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "POST", entries[0].Action)
	assert.Equal(t, "test-resource", entries[0].Resource)
	assert.Equal(t, "123", entries[0].ResourceID)
}

func TestAsync(t *testing.T) {
	store := NewMemoryStore()
	l := NewLogger(store, WithAsync(100))

	for i := 0; i < 10; i++ {
		err := l.Log(t.Context(), &Entry{Actor: "alice", Action: "create", Resource: "order"})
		require.NoError(t, err)
	}

	// 等待异步写入完成
	time.Sleep(100 * time.Millisecond)

	entries, err := l.Query(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, entries, 10)
}
