package pgvector_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/pgvector"
)

func setupPgvector(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb", host, port.Port())

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

func TestPgvectorStore_AddAndSearch(t *testing.T) {
	pool, cleanup := setupPgvector(t)
	defer cleanup()
	ctx := context.Background()

	store, err := pgvector.New(ctx, pgvector.Config{Pool: pool, Table: "test_docs", Dimension: 3})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))

	docs := []vectorstore.Document{
		{ID: "1", Content: "Go 并发", Vector: []float32{1, 0, 0}},
		{ID: "2", Content: "Python 异步", Vector: []float32{0, 1, 0}},
		{ID: "3", Content: "Go 接口", Vector: []float32{0.9, 0.1, 0}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))

	results, err := store.SimilaritySearch(ctx, []float32{1, 0, 0}, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "1", results[0].Document.ID)
}

func TestPgvectorStore_Delete(t *testing.T) {
	pool, cleanup := setupPgvector(t)
	defer cleanup()
	ctx := context.Background()

	store, err := pgvector.New(ctx, pgvector.Config{Pool: pool, Table: "del_docs", Dimension: 2})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))

	docs := []vectorstore.Document{
		{ID: "a", Content: "doc a", Vector: []float32{1, 0}},
		{ID: "b", Content: "doc b", Vector: []float32{0, 1}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))
	require.NoError(t, store.Delete(ctx, []string{"a"}))

	results, err := store.SimilaritySearch(ctx, []float32{1, 0}, 10)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "a", r.Document.ID)
	}
}

func TestPgvectorStore_AutoMigrate_Idempotent(t *testing.T) {
	pool, cleanup := setupPgvector(t)
	defer cleanup()
	ctx := context.Background()

	store, err := pgvector.New(ctx, pgvector.Config{Pool: pool, Table: "idem_docs", Dimension: 2})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))
	require.NoError(t, store.AutoMigrate(ctx))
}
