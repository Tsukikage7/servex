//go:build integration

package integration

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/storage/cache"
	cacheredis "github.com/Tsukikage7/servex/v2/storage/cache/redis"
	"github.com/Tsukikage7/servex/v2/testx"
)

func newRedisCache(t *testing.T) cache.Cache {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	cfg := cache.NewRedisConfig(addr)
	c, err := cacheredis.NewRedisCache(cfg, testx.NopLogger())
	if err != nil {
		t.Skipf("Redis cache not available: %v", err)
		return nil
	}

	t.Cleanup(func() { c.Close() })
	return c
}

func TestCache_Integration(t *testing.T) {
	c := newRedisCache(t)
	ctx := t.Context()

	t.Run("SetGetDel", func(t *testing.T) {
		key := testKey("cache:basic")
		t.Cleanup(func() { c.Del(ctx, key) })

		err := c.Set(ctx, key, "hello", time.Minute)
		require.NoError(t, err)

		val, err := c.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "hello", val)

		exists, err := c.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists)

		err = c.Del(ctx, key)
		require.NoError(t, err)

		_, err = c.Get(ctx, key)
		assert.ErrorIs(t, err, cache.ErrNotFound)
	})

	t.Run("SetNX", func(t *testing.T) {
		key := testKey("cache:setnx")
		t.Cleanup(func() { c.Del(ctx, key) })

		ok, err := c.SetNX(ctx, key, "first", time.Minute)
		require.NoError(t, err)
		assert.True(t, ok)

		ok, err = c.SetNX(ctx, key, "second", time.Minute)
		require.NoError(t, err)
		assert.False(t, ok)

		val, err := c.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "first", val)
	})

	t.Run("Increment", func(t *testing.T) {
		key := testKey("cache:incr")
		t.Cleanup(func() { c.Del(ctx, key) })

		v, err := c.Increment(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(1), v)

		v, err = c.IncrementBy(ctx, key, 5)
		require.NoError(t, err)
		assert.Equal(t, int64(6), v)

		v, err = c.Decrement(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(5), v)
	})

	t.Run("Expire_TTL", func(t *testing.T) {
		key := testKey("cache:ttl")
		t.Cleanup(func() { c.Del(ctx, key) })

		err := c.Set(ctx, key, "val", time.Minute)
		require.NoError(t, err)

		ttl, err := c.TTL(ctx, key)
		require.NoError(t, err)
		assert.True(t, ttl > 0)

		err = c.Expire(ctx, key, 10*time.Second)
		require.NoError(t, err)

		ttl, err = c.TTL(ctx, key)
		require.NoError(t, err)
		assert.True(t, ttl > 0 && ttl <= 10*time.Second)
	})

	t.Run("MGetMSet", func(t *testing.T) {
		key1 := testKey("cache:mset:1")
		key2 := testKey("cache:mset:2")
		t.Cleanup(func() { c.Del(ctx, key1, key2) })

		err := c.MSet(ctx, map[string]any{
			key1: "v1",
			key2: "v2",
		}, time.Minute)
		require.NoError(t, err)

		vals, err := c.MGet(ctx, key1, key2)
		require.NoError(t, err)
		assert.Equal(t, "v1", vals[0])
		assert.Equal(t, "v2", vals[1])
	})

	t.Run("Lock_Unlock", func(t *testing.T) {
		key := testKey("cache:lock")
		t.Cleanup(func() { c.Del(ctx, key) })

		ok, err := c.TryLock(ctx, key, "owner1", 10*time.Second)
		require.NoError(t, err)
		assert.True(t, ok)

		// Same key, different owner -> should fail
		ok, err = c.TryLock(ctx, key, "owner2", 10*time.Second)
		require.NoError(t, err)
		assert.False(t, ok)

		// Unlock with correct owner
		err = c.Unlock(ctx, key, "owner1")
		require.NoError(t, err)

		// Now another owner can acquire
		ok, err = c.TryLock(ctx, key, "owner2", 10*time.Second)
		require.NoError(t, err)
		assert.True(t, ok)

		err = c.Unlock(ctx, key, "owner2")
		require.NoError(t, err)
	})
}
