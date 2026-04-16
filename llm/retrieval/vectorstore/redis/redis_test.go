package redis_test

import (
	"context"
	"fmt"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
	redisvs "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/redis"
)

func setupRedisStack(t *testing.T) (*goredis.Client, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis/redis-stack:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "6379")

	client := goredis.NewClient(&goredis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})

	return client, func() {
		_ = client.Close()
		_ = container.Terminate(ctx)
	}
}

func TestRedisStore_AddAndSearch(t *testing.T) {
	client, cleanup := setupRedisStack(t)
	defer cleanup()
	ctx := context.Background()

	store, err := redisvs.New(ctx, redisvs.Config{
		Client:    client,
		IndexName: "test_idx",
		Dimension: 3,
	})
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

func TestRedisStore_Delete(t *testing.T) {
	client, cleanup := setupRedisStack(t)
	defer cleanup()
	ctx := context.Background()

	store, err := redisvs.New(ctx, redisvs.Config{Client: client, IndexName: "del_idx", Dimension: 2})
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

func TestRedisStore_AutoMigrate_Idempotent(t *testing.T) {
	client, cleanup := setupRedisStack(t)
	defer cleanup()
	ctx := context.Background()

	store, err := redisvs.New(ctx, redisvs.Config{Client: client, IndexName: "idem_idx", Dimension: 2})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))
	require.NoError(t, store.AutoMigrate(ctx))
}
