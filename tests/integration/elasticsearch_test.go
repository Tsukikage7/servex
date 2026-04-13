//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/storage/elasticsearch"
	"github.com/Tsukikage7/servex/v2/testx"
)

func esAddresses() []string {
	addr := os.Getenv("ES_ADDRESSES")
	if addr == "" {
		addr = "http://localhost:9200"
	}
	return strings.Split(addr, ",")
}

func newESClient(t *testing.T) elasticsearch.Client {
	t.Helper()

	cfg := elasticsearch.DefaultConfig()
	cfg.Addresses = esAddresses()

	client, err := elasticsearch.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Skipf("Elasticsearch not available: %v", err)
		return nil
	}

	t.Cleanup(func() { client.Close() })
	return client
}

func testIndexName() string {
	return fmt.Sprintf("servex_inttest_%d", time.Now().UnixNano())
}

func TestElasticsearch_Integration(t *testing.T) {
	client := newESClient(t)
	ctx := t.Context()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		require.NoError(t, err)
	})

	t.Run("Index_CRUD", func(t *testing.T) {
		idxName := testIndexName()
		idx := client.Index(idxName)
		t.Cleanup(func() { idx.Delete(ctx) })

		// Create index
		err := idx.Create(ctx, map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"title":   map[string]any{"type": "text"},
					"status":  map[string]any{"type": "keyword"},
					"score":   map[string]any{"type": "integer"},
					"created": map[string]any{"type": "date"},
				},
			},
		})
		require.NoError(t, err)

		// Exists
		exists, err := idx.Exists(ctx)
		require.NoError(t, err)
		assert.True(t, exists)

		// GetMapping
		mapping, err := idx.GetMapping(ctx)
		require.NoError(t, err)
		assert.NotNil(t, mapping)

		// Delete
		err = idx.Delete(ctx)
		require.NoError(t, err)

		exists, err = idx.Exists(ctx)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Document_CRUD", func(t *testing.T) {
		idxName := testIndexName()
		idx := client.Index(idxName)
		t.Cleanup(func() { idx.Delete(ctx) })

		// Create index first
		err := idx.Create(ctx, map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"title": map[string]any{"type": "text"},
					"score": map[string]any{"type": "integer"},
				},
			},
		})
		require.NoError(t, err)

		doc := idx.Document()

		// Index document
		indexResult, err := doc.Index(ctx, "1", map[string]any{
			"title": "Hello World",
			"score": 100,
		})
		require.NoError(t, err)
		assert.Equal(t, "1", indexResult.ID)

		// Get document
		getResult, err := doc.Get(ctx, "1")
		require.NoError(t, err)
		assert.True(t, getResult.Found)
		assert.Equal(t, "1", getResult.ID)

		var source map[string]any
		err = json.Unmarshal(getResult.Source, &source)
		require.NoError(t, err)
		assert.Equal(t, "Hello World", source["title"])

		// Update document (不需要包 "doc" 包装，Update 方法内部会自动包装)
		updateResult, err := doc.Update(ctx, "1", map[string]any{"score": 200})
		require.NoError(t, err)
		assert.Equal(t, "1", updateResult.ID)

		// Verify update
		getResult, err = doc.Get(ctx, "1")
		require.NoError(t, err)
		err = json.Unmarshal(getResult.Source, &source)
		require.NoError(t, err)
		assert.Equal(t, float64(200), source["score"])

		// Exists
		exists, err := doc.Exists(ctx, "1")
		require.NoError(t, err)
		assert.True(t, exists)

		exists, err = doc.Exists(ctx, "nonexistent")
		require.NoError(t, err)
		assert.False(t, exists)

		// Delete document
		delResult, err := doc.Delete(ctx, "1")
		require.NoError(t, err)
		assert.Equal(t, "1", delResult.ID)

		// Verify deletion
		exists, err = doc.Exists(ctx, "1")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Search", func(t *testing.T) {
		idxName := testIndexName()
		idx := client.Index(idxName)
		t.Cleanup(func() { idx.Delete(ctx) })

		// Create index
		err := idx.Create(ctx, map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"title":  map[string]any{"type": "text"},
					"status": map[string]any{"type": "keyword"},
					"score":  map[string]any{"type": "integer"},
				},
			},
		})
		require.NoError(t, err)

		doc := idx.Document()

		// Index test documents
		for i, d := range []map[string]any{
			{"title": "Go programming", "status": "published", "score": 10},
			{"title": "Go concurrency patterns", "status": "published", "score": 20},
			{"title": "Python basics", "status": "draft", "score": 5},
		} {
			_, err := doc.Index(ctx, fmt.Sprintf("%d", i+1), d)
			require.NoError(t, err)
		}

		// Wait for indexing to complete (ES is near-realtime)
		time.Sleep(1500 * time.Millisecond)

		search := idx.Search()

		// Match query
		result, err := search.Query(ctx, map[string]any{
			"match": map[string]any{
				"title": "Go",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.TotalHits)

		// Term query
		result, err = search.Query(ctx, map[string]any{
			"term": map[string]any{
				"status": "draft",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalHits)

		// Count
		count, err := search.Count(ctx, map[string]any{
			"match_all": map[string]any{},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)

		// Search with size option
		result, err = search.Query(ctx, map[string]any{
			"match_all": map[string]any{},
		}, elasticsearch.WithSize(1))
		require.NoError(t, err)
		assert.Len(t, result.Hits, 1)
	})

	t.Run("Aggregation", func(t *testing.T) {
		idxName := testIndexName()
		idx := client.Index(idxName)
		t.Cleanup(func() { idx.Delete(ctx) })

		// Create index with keyword field
		err := idx.Create(ctx, map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"category": map[string]any{"type": "keyword"},
					"price":    map[string]any{"type": "float"},
				},
			},
		})
		require.NoError(t, err)

		doc := idx.Document()
		for i, d := range []map[string]any{
			{"category": "books", "price": 10.0},
			{"category": "books", "price": 20.0},
			{"category": "electronics", "price": 100.0},
		} {
			_, err := doc.Index(ctx, fmt.Sprintf("%d", i+1), d)
			require.NoError(t, err)
		}

		time.Sleep(1500 * time.Millisecond)

		search := idx.Search()

		result, err := search.Aggregate(ctx, map[string]any{
			"by_category": map[string]any{
				"terms": map[string]any{
					"field": "category",
				},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, result.Aggregations)
		assert.Contains(t, result.Aggregations, "by_category")
	})

	t.Run("Bulk", func(t *testing.T) {
		idxName := testIndexName()
		idx := client.Index(idxName)
		t.Cleanup(func() { idx.Delete(ctx) })

		err := idx.Create(ctx, map[string]any{
			"settings": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
		})
		require.NoError(t, err)

		doc := idx.Document()

		actions := []elasticsearch.BulkAction{
			{Type: "index", ID: "b1", Body: map[string]any{"title": "Bulk Doc 1"}},
			{Type: "index", ID: "b2", Body: map[string]any{"title": "Bulk Doc 2"}},
			{Type: "index", ID: "b3", Body: map[string]any{"title": "Bulk Doc 3"}},
		}

		bulkResult, err := doc.Bulk(ctx, actions)
		require.NoError(t, err)
		assert.False(t, bulkResult.Errors)
		assert.Len(t, bulkResult.Items, 3)

		// Verify
		for _, id := range []string{"b1", "b2", "b3"} {
			exists, err := doc.Exists(ctx, id)
			require.NoError(t, err)
			assert.True(t, exists)
		}
	})
}
