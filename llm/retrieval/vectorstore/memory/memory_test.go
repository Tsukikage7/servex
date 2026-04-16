package memory_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore/memory"
)

func TestMemoryStore_AddAndSearch(t *testing.T) {
	store := memory.New()
	ctx := context.Background()

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
	assert.Equal(t, "3", results[1].Document.ID)
}

func TestMemoryStore_ScoreThreshold(t *testing.T) {
	store := memory.New()
	ctx := context.Background()

	docs := []vectorstore.Document{
		{ID: "1", Content: "close", Vector: []float32{1, 0}},
		{ID: "2", Content: "far", Vector: []float32{0, 1}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))

	results, err := store.SimilaritySearch(ctx, []float32{1, 0}, 10,
		vectorstore.WithScoreThreshold(0.9))
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "1", results[0].Document.ID)
}

func TestMemoryStore_MetadataFilter(t *testing.T) {
	store := memory.New()
	ctx := context.Background()

	docs := []vectorstore.Document{
		{ID: "1", Content: "go doc", Vector: []float32{1, 0}, Metadata: map[string]any{"lang": "go"}},
		{ID: "2", Content: "py doc", Vector: []float32{1, 0}, Metadata: map[string]any{"lang": "python"}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))

	results, err := store.SimilaritySearch(ctx, []float32{1, 0}, 10,
		vectorstore.WithFilter(map[string]any{"lang": "go"}))
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "1", results[0].Document.ID)
}

func TestMemoryStore_Delete(t *testing.T) {
	store := memory.New()
	ctx := context.Background()

	docs := []vectorstore.Document{
		{ID: "1", Content: "doc1", Vector: []float32{1, 0}},
		{ID: "2", Content: "doc2", Vector: []float32{0, 1}},
	}
	require.NoError(t, store.AddDocuments(ctx, docs))
	require.NoError(t, store.Delete(ctx, []string{"1"}))

	results, err := store.SimilaritySearch(ctx, []float32{1, 0}, 10)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "1", r.Document.ID)
	}
}
