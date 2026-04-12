package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/observability/logger"
)

// redisAddr 获取 Redis 地址.
func redisAddr() string {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return addr
}

// testLog 测试用日志.
var testLog logger.Logger

func TestMain(m *testing.M) {
	var err error
	testLog, err = logger.NewLogger(&logger.Config{
		Level:  "error",
		Format: "console",
		Output: "console",
	})
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// skipIfNoRedis 如果无法连接 Redis 则跳过测试.
func skipIfNoRedis(t *testing.T) Client {
	t.Helper()

	cfg := DefaultConfig()
	cfg.Addr = redisAddr()

	client, err := NewClient(cfg, testLog)
	if err != nil {
		t.Skipf("跳过 Redis 集成测试: %v", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		client.Close()
		t.Skipf("跳过 Redis 集成测试: 无法连接 %s: %v", cfg.Addr, err)
		return nil
	}

	t.Cleanup(func() { client.Close() })
	return client
}

// ============================================================================
// 单元测试
// ============================================================================

func TestNewClient_Validation(t *testing.T) {
	// nil config
	_, err := NewClient(nil, testLog)
	assert.ErrorIs(t, err, ErrNilConfig)

	// nil logger
	_, err = NewClient(DefaultConfig(), nil)
	assert.ErrorIs(t, err, ErrNilLogger)

}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "localhost:6379", cfg.Addr)
	assert.Equal(t, 0, cfg.DB)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 10, cfg.PoolSize)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
	assert.Equal(t, 3*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 3*time.Second, cfg.WriteTimeout)
}

func TestConfig_ApplyDefaults(t *testing.T) {
	cfg := &Config{Addr: "custom:6380"}
	cfg.ApplyDefaults()
	assert.Equal(t, "custom:6380", cfg.Addr) // 不覆盖已有值
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 10, cfg.PoolSize)
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{Addr: ""}
	assert.ErrorIs(t, cfg.Validate(), ErrEmptyAddr)

	cfg.Addr = "localhost:6379"
	assert.NoError(t, cfg.Validate())
}

// ============================================================================
// 集成测试
// ============================================================================

func TestPing(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short 模式）")
	}
	client := skipIfNoRedis(t)
	err := client.Ping(t.Context())
	assert.NoError(t, err)
}

func TestStringOps(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short 模式）")
	}
	client := skipIfNoRedis(t)
	ctx := t.Context()
	key := "servex:test:string:" + time.Now().Format("20060102150405.000")

	t.Cleanup(func() { client.Del(ctx, key) })

	// Set & Get
	err := client.Set(ctx, key, "hello", time.Minute)
	require.NoError(t, err)

	val, err := client.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "hello", val)

	// Exists
	n, err := client.Exists(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Incr
	incrKey := key + ":incr"
	t.Cleanup(func() { client.Del(ctx, incrKey) })

	v, err := client.Incr(ctx, incrKey)
	require.NoError(t, err)
	assert.Equal(t, int64(1), v)

	v, err = client.IncrBy(ctx, incrKey, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), v)

	// TTL
	ttl, err := client.TTL(ctx, key)
	require.NoError(t, err)
	assert.True(t, ttl > 0)

	// Del
	deleted, err := client.Del(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Get 不存在的 key
	_, err = client.Get(ctx, key)
	assert.ErrorIs(t, err, goredis.Nil)
}

func TestHashOps(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short 模式）")
	}
	client := skipIfNoRedis(t)
	ctx := t.Context()
	key := "servex:test:hash:" + time.Now().Format("20060102150405.000")

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
	assert.ErrorIs(t, err, goredis.Nil)
}

func TestListOps(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short 模式）")
	}
	client := skipIfNoRedis(t)
	ctx := t.Context()
	key := "servex:test:list:" + time.Now().Format("20060102150405.000")

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
}

func TestSortedSetOps(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short 模式）")
	}
	client := skipIfNoRedis(t)
	ctx := t.Context()
	key := "servex:test:zset:" + time.Now().Format("20060102150405.000")

	t.Cleanup(func() { client.Del(ctx, key) })

	// ZAdd
	_, err := client.ZAdd(ctx, key,
		Z{Score: 1, Member: "a"},
		Z{Score: 2, Member: "b"},
		Z{Score: 3, Member: "c"},
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
}
