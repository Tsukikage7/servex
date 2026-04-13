//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/storage/mongodb"
	"github.com/Tsukikage7/servex/v2/testx"
)

func mongoURI() string {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	return uri
}

func newMongoClient(t *testing.T) mongodb.Client {
	t.Helper()

	cfg := mongodb.DefaultConfig()
	cfg.URI = mongoURI()
	cfg.Database = fmt.Sprintf("servex_inttest_%d", time.Now().UnixNano())

	client, err := mongodb.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
		return nil
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		// Drop the test database
		client.Database().Drop(ctx)
		client.Close(ctx)
	})

	return client
}

func TestMongoDB_Integration(t *testing.T) {
	client := newMongoClient(t)
	ctx := t.Context()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		require.NoError(t, err)
	})

	t.Run("CRUD", func(t *testing.T) {
		collName := fmt.Sprintf("test_crud_%d", time.Now().UnixNano())
		coll := client.Collection(collName)
		t.Cleanup(func() { coll.Drop(ctx) })

		// InsertOne
		doc := mongodb.M{"name": "alice", "age": 30}
		result, err := coll.InsertOne(ctx, doc)
		require.NoError(t, err)
		assert.NotNil(t, result.InsertedID)

		// FindOne
		var found mongodb.M
		err = coll.FindOne(ctx, mongodb.M{"name": "alice"}).Decode(&found)
		require.NoError(t, err)
		assert.Equal(t, "alice", found["name"])
		assert.Equal(t, int32(30), found["age"])

		// UpdateOne
		updateResult, err := coll.UpdateOne(ctx,
			mongodb.M{"name": "alice"},
			mongodb.M{"$set": mongodb.M{"age": 31}},
		)
		require.NoError(t, err)
		assert.Equal(t, int64(1), updateResult.ModifiedCount)

		// Verify update
		err = coll.FindOne(ctx, mongodb.M{"name": "alice"}).Decode(&found)
		require.NoError(t, err)
		assert.Equal(t, int32(31), found["age"])

		// DeleteOne
		delResult, err := coll.DeleteOne(ctx, mongodb.M{"name": "alice"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), delResult.DeletedCount)

		// Verify deletion
		count, err := coll.CountDocuments(ctx, mongodb.M{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("InsertMany_Find", func(t *testing.T) {
		collName := fmt.Sprintf("test_many_%d", time.Now().UnixNano())
		coll := client.Collection(collName)
		t.Cleanup(func() { coll.Drop(ctx) })

		docs := []any{
			mongodb.M{"name": "a", "score": 10},
			mongodb.M{"name": "b", "score": 20},
			mongodb.M{"name": "c", "score": 30},
		}
		result, err := coll.InsertMany(ctx, docs)
		require.NoError(t, err)
		assert.Len(t, result.InsertedIDs, 3)

		// Find all
		cursor, err := coll.Find(ctx, mongodb.M{}, mongodb.WithFindSort(mongodb.M{"score": 1}))
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []mongodb.M
		err = cursor.All(ctx, &results)
		require.NoError(t, err)
		assert.Len(t, results, 3)
		assert.Equal(t, "a", results[0]["name"])
		assert.Equal(t, "c", results[2]["name"])

		// CountDocuments
		count, err := coll.CountDocuments(ctx, mongodb.M{})
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)

		// Find with limit
		cursor, err = coll.Find(ctx, mongodb.M{}, mongodb.WithFindLimit(2))
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var limited []mongodb.M
		err = cursor.All(ctx, &limited)
		require.NoError(t, err)
		assert.Len(t, limited, 2)
	})

	t.Run("Index", func(t *testing.T) {
		collName := fmt.Sprintf("test_index_%d", time.Now().UnixNano())
		coll := client.Collection(collName)
		t.Cleanup(func() { coll.Drop(ctx) })

		// Insert a doc first so the collection exists
		_, err := coll.InsertOne(ctx, mongodb.M{"email": "test@example.com"})
		require.NoError(t, err)

		// CreateIndex
		idxName, err := coll.CreateIndex(ctx, mongodb.IndexModel{
			Keys: mongodb.D{{Key: "email", Value: 1}},
			Options: &mongodb.IndexOptions{
				Name:   "idx_email",
				Unique: true,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "idx_email", idxName)

		// ListIndexes
		cursor, err := coll.ListIndexes(ctx)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var indexes []mongodb.M
		err = cursor.All(ctx, &indexes)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(indexes), 2) // _id + idx_email

		// DropIndex
		err = coll.DropIndex(ctx, "idx_email")
		require.NoError(t, err)
	})

	t.Run("FindOneAndUpdate", func(t *testing.T) {
		collName := fmt.Sprintf("test_fau_%d", time.Now().UnixNano())
		coll := client.Collection(collName)
		t.Cleanup(func() { coll.Drop(ctx) })

		_, err := coll.InsertOne(ctx, mongodb.M{"name": "bob", "visits": 0})
		require.NoError(t, err)

		var updated mongodb.M
		err = coll.FindOneAndUpdate(ctx,
			mongodb.M{"name": "bob"},
			mongodb.M{"$inc": mongodb.M{"visits": 1}},
		).Decode(&updated)
		require.NoError(t, err)
		// FindOneAndUpdate returns the document BEFORE the update by default
		assert.Equal(t, int32(0), updated["visits"])
	})

	t.Run("DeleteMany", func(t *testing.T) {
		collName := fmt.Sprintf("test_delmany_%d", time.Now().UnixNano())
		coll := client.Collection(collName)
		t.Cleanup(func() { coll.Drop(ctx) })

		docs := []any{
			mongodb.M{"status": "old"},
			mongodb.M{"status": "old"},
			mongodb.M{"status": "new"},
		}
		_, err := coll.InsertMany(ctx, docs)
		require.NoError(t, err)

		delResult, err := coll.DeleteMany(ctx, mongodb.M{"status": "old"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), delResult.DeletedCount)

		count, err := coll.CountDocuments(ctx, mongodb.M{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}
