//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storeredis "github.com/Tsukikage7/servex/v2/storage/redis"
	"github.com/Tsukikage7/servex/v2/testx"
)

func redisAddr() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return addr
}

func newRedisClient(t *testing.T) storeredis.Client {
	t.Helper()

	cfg := storeredis.DefaultConfig()
	cfg.Addr = redisAddr()

	client, err := storeredis.NewClient(cfg, testx.NopLogger())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		client.Close()
		t.Skipf("Redis not available at %s: %v", cfg.Addr, err)
		return nil
	}

	t.Cleanup(func() { client.Close() })
	return client
}

func testKey(prefix string) string {
	return fmt.Sprintf("servex:inttest:%s:%d", prefix, time.Now().UnixNano())
}

func TestRedis_Integration(t *testing.T) {
	client := newRedisClient(t)
	ctx := t.Context()

	t.Run("String", func(t *testing.T) {
		key := testKey("string")
		t.Cleanup(func() { client.Del(ctx, key) })

		// Set & Get
		err := client.Set(ctx, key, "hello", time.Minute)
		require.NoError(t, err)

		val, err := client.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "hello", val)

		// Overwrite
		err = client.Set(ctx, key, "world", time.Minute)
		require.NoError(t, err)

		val, err = client.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "world", val)

		// Exists
		n, err := client.Exists(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(1), n)

		// TTL
		ttl, err := client.TTL(ctx, key)
		require.NoError(t, err)
		assert.True(t, ttl > 0)

		// Expire
		ok, err := client.Expire(ctx, key, 10*time.Second)
		require.NoError(t, err)
		assert.True(t, ok)

		// Incr / IncrBy
		incrKey := testKey("string:incr")
		t.Cleanup(func() { client.Del(ctx, incrKey) })

		v, err := client.Incr(ctx, incrKey)
		require.NoError(t, err)
		assert.Equal(t, int64(1), v)

		v, err = client.IncrBy(ctx, incrKey, 5)
		require.NoError(t, err)
		assert.Equal(t, int64(6), v)

		// Del
		deleted, err := client.Del(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		// Get non-existent key
		_, err = client.Get(ctx, key)
		assert.ErrorIs(t, err, redis.Nil)
	})

	t.Run("Hash", func(t *testing.T) {
		key := testKey("hash")
		t.Cleanup(func() { client.Del(ctx, key) })

		// HSet
		_, err := client.HSet(ctx, key, "name", "alice", "age", "30")
		require.NoError(t, err)

		// HGet
		val, err := client.HGet(ctx, key, "name")
		require.NoError(t, err)
		assert.Equal(t, "alice", val)

		// HGetAll
		all, err := client.HGetAll(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "alice", all["name"])
		assert.Equal(t, "30", all["age"])

		// HDel
		_, err = client.HDel(ctx, key, "age")
		require.NoError(t, err)

		_, err = client.HGet(ctx, key, "age")
		assert.ErrorIs(t, err, redis.Nil)
	})

	t.Run("List", func(t *testing.T) {
		key := testKey("list")
		t.Cleanup(func() { client.Del(ctx, key) })

		// LPush & RPush
		_, err := client.LPush(ctx, key, "a")
		require.NoError(t, err)
		_, err = client.RPush(ctx, key, "b", "c")
		require.NoError(t, err)

		// LLen
		length, err := client.LLen(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(3), length)

		// LRange
		vals, err := client.LRange(ctx, key, 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, vals)

		// LPop & RPop
		v, err := client.LPop(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "a", v)

		v, err = client.RPop(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "c", v)
	})

	t.Run("Set", func(t *testing.T) {
		key := testKey("set")
		t.Cleanup(func() { client.Del(ctx, key) })

		// SAdd
		_, err := client.SAdd(ctx, key, "a", "b", "c")
		require.NoError(t, err)

		// SMembers
		members, err := client.SMembers(ctx, key)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"a", "b", "c"}, members)

		// SIsMember
		ok, err := client.SIsMember(ctx, key, "a")
		require.NoError(t, err)
		assert.True(t, ok)

		ok, err = client.SIsMember(ctx, key, "z")
		require.NoError(t, err)
		assert.False(t, ok)

		// SRem
		_, err = client.SRem(ctx, key, "b")
		require.NoError(t, err)

		members, err = client.SMembers(ctx, key)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"a", "c"}, members)
	})

	t.Run("SortedSet", func(t *testing.T) {
		key := testKey("zset")
		t.Cleanup(func() { client.Del(ctx, key) })

		// ZAdd
		_, err := client.ZAdd(ctx, key,
			redis.Z{Score: 1, Member: "a"},
			redis.Z{Score: 2, Member: "b"},
			redis.Z{Score: 3, Member: "c"},
		)
		require.NoError(t, err)

		// ZCard
		card, err := client.ZCard(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(3), card)

		// ZRange
		vals, err := client.ZRange(ctx, key, 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, vals)

		// ZRangeWithScores
		zs, err := client.ZRangeWithScores(ctx, key, 0, -1)
		require.NoError(t, err)
		assert.Len(t, zs, 3)

		// ZScore
		score, err := client.ZScore(ctx, key, "b")
		require.NoError(t, err)
		assert.Equal(t, float64(2), score)

		// ZRank
		rank, err := client.ZRank(ctx, key, "a")
		require.NoError(t, err)
		assert.Equal(t, int64(0), rank)

		// ZRem
		_, err = client.ZRem(ctx, key, "b")
		require.NoError(t, err)

		card, err = client.ZCard(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(2), card)
	})

	t.Run("Pipeline", func(t *testing.T) {
		key1 := testKey("pipe:1")
		key2 := testKey("pipe:2")
		t.Cleanup(func() { client.Del(ctx, key1, key2) })

		err := client.PipelineExec(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key1, "v1", time.Minute)
			pipe.Set(ctx, key2, "v2", time.Minute)
			return nil
		})
		require.NoError(t, err)

		val1, err := client.Get(ctx, key1)
		require.NoError(t, err)
		assert.Equal(t, "v1", val1)

		val2, err := client.Get(ctx, key2)
		require.NoError(t, err)
		assert.Equal(t, "v2", val2)
	})
}
