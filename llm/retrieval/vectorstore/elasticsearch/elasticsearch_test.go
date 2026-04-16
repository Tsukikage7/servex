package elasticsearch_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
	esvectorstore "github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/elasticsearch"
)

func setupES(t *testing.T) (*esv8.Client, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "docker.elastic.co/elasticsearch/elasticsearch:8.15.0",
		ExposedPorts: []string{"9200/tcp"},
		Env: map[string]string{
			"discovery.type":           "single-node",
			"xpack.security.enabled":   "false",
			"ES_JAVA_OPTS":             "-Xms512m -Xmx512m",
		},
		WaitingFor: wait.ForHTTP("/").WithPort("9200/tcp").WithStartupTimeout(120 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "9200")
	require.NoError(t, err)

	addr := fmt.Sprintf("http://%s:%s", host, port.Port())
	client, err := esv8.NewClient(esv8.Config{
		Addresses: []string{addr},
	})
	require.NoError(t, err)

	return client, func() {
		_ = container.Terminate(ctx)
	}
}

func TestESStore_AddAndSearch(t *testing.T) {
	client, cleanup := setupES(t)
	defer cleanup()
	ctx := context.Background()

	store, err := esvectorstore.New(ctx, esvectorstore.Config{
		Client:    client,
		IndexName: "test_add_search",
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

	// ES 索引刷新需要短暂等待
	time.Sleep(1500 * time.Millisecond)

	results, err := store.SimilaritySearch(ctx, []float32{1, 0, 0}, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "1", results[0].Document.ID)
}

func TestESStore_Delete(t *testing.T) {
	client, cleanup := setupES(t)
	defer cleanup()
	ctx := context.Background()

	store, err := esvectorstore.New(ctx, esvectorstore.Config{
		Client:    client,
		IndexName: "test_delete",
		Dimension: 2,
	})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))

	docs := []vectorstore.Document{
		{ID: "a", Content: "doc a", Vector: []float32{1, 0}},
		{ID: "b", Content: "doc b", Vector: []float32{0, 1}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))

	time.Sleep(1500 * time.Millisecond)

	require.NoError(t, store.Delete(ctx, []string{"a"}))

	time.Sleep(1500 * time.Millisecond)

	results, err := store.SimilaritySearch(ctx, []float32{1, 0}, 10)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "a", r.Document.ID)
	}
}

func TestESStore_AutoMigrate_Idempotent(t *testing.T) {
	client, cleanup := setupES(t)
	defer cleanup()
	ctx := context.Background()

	store, err := esvectorstore.New(ctx, esvectorstore.Config{
		Client:    client,
		IndexName: "test_idempotent",
		Dimension: 2,
	})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))
	require.NoError(t, store.AutoMigrate(ctx))
}

func TestESStore_HybridSearch(t *testing.T) {
	client, cleanup := setupES(t)
	defer cleanup()
	ctx := context.Background()

	store, err := esvectorstore.New(ctx, esvectorstore.Config{
		Client:    client,
		IndexName: "test_hybrid",
		Dimension: 3,
	})
	require.NoError(t, err)
	require.NoError(t, store.AutoMigrate(ctx))

	docs := []vectorstore.Document{
		{ID: "1", Content: "Go 并发编程指南", Vector: []float32{1, 0, 0}},
		{ID: "2", Content: "Python 机器学习", Vector: []float32{0, 1, 0}},
		{ID: "3", Content: "Go 微服务架构", Vector: []float32{0.8, 0.2, 0}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))

	time.Sleep(1500 * time.Millisecond)

	results, err := store.SimilaritySearch(ctx, []float32{1, 0, 0}, 3,
		vectorstore.WithTextQuery("Go 并发"))
	require.NoError(t, err)
	require.NotEmpty(t, results)
	// 混合搜索结果包含 Go 相关文档
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.Document.ID)
	}
	assert.Contains(t, ids, "1")
}
