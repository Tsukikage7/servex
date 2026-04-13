package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// MemoryCacheTestSuite 内存缓存测试套件.
type MemoryCacheTestSuite struct {
	suite.Suite
	cache  *memoryCache
	ctx    context.Context
	logger logger.Logger
}

func TestMemoryCacheSuite(t *testing.T) {
	suite.Run(t, new(MemoryCacheTestSuite))
}

func (s *MemoryCacheTestSuite) SetupSuite() {
	log, err := logger.NewLogger(logger.DefaultConfig())
	s.Require().NoError(err)
	s.logger = log
}

func (s *MemoryCacheTestSuite) TearDownSuite() {
	if s.logger != nil {
		s.logger.Close()
	}
}

func (s *MemoryCacheTestSuite) SetupTest() {
	config := NewMemoryConfig()
	config.CleanupInterval = 100 * time.Millisecond
	cache, err := NewMemoryCache(config, s.logger)
	s.Require().NoError(err)
	s.cache = cache.(*memoryCache)
	s.ctx = s.T().Context()
}

func (s *MemoryCacheTestSuite) TearDownTest() {
	if s.cache != nil {
		s.cache.Close()
	}
}

func (s *MemoryCacheTestSuite) TestNewMemoryCache() {
	cache, err := NewMemoryCache(nil, s.logger)
	s.NoError(err)
	s.NotNil(cache)
	defer cache.Close()
}

func (s *MemoryCacheTestSuite) TestSetAndGet() {
	err := s.cache.Set(s.ctx, "key1", "value1", time.Minute)
	s.NoError(err)

	value, err := s.cache.Get(s.ctx, "key1")
	s.NoError(err)
	s.Equal("value1", value)
}

func (s *MemoryCacheTestSuite) TestSetAndGet_ByteSlice() {
	err := s.cache.Set(s.ctx, "key1", []byte("bytes"), time.Minute)
	s.NoError(err)

	value, err := s.cache.Get(s.ctx, "key1")
	s.NoError(err)
	s.Equal("bytes", value)
}

func (s *MemoryCacheTestSuite) TestSetAndGet_JSON() {
	data := map[string]int{"a": 1, "b": 2}
	err := s.cache.Set(s.ctx, "key1", data, time.Minute)
	s.NoError(err)

	value, err := s.cache.Get(s.ctx, "key1")
	s.NoError(err)
	s.Contains(value, `"a":1`)
}

func (s *MemoryCacheTestSuite) TestGet_NotFound() {
	_, err := s.cache.Get(s.ctx, "nonexistent")
	s.Equal(ErrNotFound, err)
}

func (s *MemoryCacheTestSuite) TestGet_Expired() {
	err := s.cache.Set(s.ctx, "key1", "value1", 10*time.Millisecond)
	s.NoError(err)

	time.Sleep(20 * time.Millisecond)

	_, err = s.cache.Get(s.ctx, "key1")
	s.Equal(ErrNotFound, err)
}

func (s *MemoryCacheTestSuite) TestSet_NoExpire() {
	err := s.cache.Set(s.ctx, "key1", "value1", 0)
	s.NoError(err)

	value, err := s.cache.Get(s.ctx, "key1")
	s.NoError(err)
	s.Equal("value1", value)
}

func (s *MemoryCacheTestSuite) TestDel() {
	s.cache.Set(s.ctx, "key1", "value1", time.Minute)
	s.cache.Set(s.ctx, "key2", "value2", time.Minute)

	err := s.cache.Del(s.ctx, "key1", "key2")
	s.NoError(err)

	_, err = s.cache.Get(s.ctx, "key1")
	s.Equal(ErrNotFound, err)

	_, err = s.cache.Get(s.ctx, "key2")
	s.Equal(ErrNotFound, err)
}

func (s *MemoryCacheTestSuite) TestExists() {
	s.cache.Set(s.ctx, "key1", "value1", time.Minute)

	exists, err := s.cache.Exists(s.ctx, "key1")
	s.NoError(err)
	s.True(exists)

	exists, err = s.cache.Exists(s.ctx, "nonexistent")
	s.NoError(err)
	s.False(exists)
}

func (s *MemoryCacheTestSuite) TestExists_Expired() {
	s.cache.Set(s.ctx, "key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	exists, err := s.cache.Exists(s.ctx, "key1")
	s.NoError(err)
	s.False(exists)
}

func (s *MemoryCacheTestSuite) TestSetNX() {
	ok, err := s.cache.SetNX(s.ctx, "key1", "value1", time.Minute)
	s.NoError(err)
	s.True(ok)

	ok, err = s.cache.SetNX(s.ctx, "key1", "value2", time.Minute)
	s.NoError(err)
	s.False(ok)

	value, _ := s.cache.Get(s.ctx, "key1")
	s.Equal("value1", value)
}

func (s *MemoryCacheTestSuite) TestSetNX_AfterExpire() {
	s.cache.SetNX(s.ctx, "key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	ok, err := s.cache.SetNX(s.ctx, "key1", "value2", time.Minute)
	s.NoError(err)
	s.True(ok)
}

func (s *MemoryCacheTestSuite) TestIncrement() {
	val, err := s.cache.Increment(s.ctx, "counter")
	s.NoError(err)
	s.Equal(int64(1), val)

	val, err = s.cache.Increment(s.ctx, "counter")
	s.NoError(err)
	s.Equal(int64(2), val)
}

func (s *MemoryCacheTestSuite) TestIncrementBy() {
	val, err := s.cache.IncrementBy(s.ctx, "counter", 5)
	s.NoError(err)
	s.Equal(int64(5), val)

	val, err = s.cache.IncrementBy(s.ctx, "counter", 3)
	s.NoError(err)
	s.Equal(int64(8), val)
}

func (s *MemoryCacheTestSuite) TestDecrement() {
	s.cache.Set(s.ctx, "counter", "10", time.Minute)

	val, err := s.cache.Decrement(s.ctx, "counter")
	s.NoError(err)
	s.Equal(int64(9), val)
}

func (s *MemoryCacheTestSuite) TestIncrement_NonInteger() {
	s.cache.Set(s.ctx, "key1", "not-a-number", time.Minute)

	_, err := s.cache.Increment(s.ctx, "key1")
	s.Error(err)
}

func (s *MemoryCacheTestSuite) TestExpire() {
	s.cache.Set(s.ctx, "key1", "value1", time.Hour)

	err := s.cache.Expire(s.ctx, "key1", 10*time.Millisecond)
	s.NoError(err)

	time.Sleep(20 * time.Millisecond)

	_, err = s.cache.Get(s.ctx, "key1")
	s.Equal(ErrNotFound, err)
}

func (s *MemoryCacheTestSuite) TestExpire_NotFound() {
	err := s.cache.Expire(s.ctx, "nonexistent", time.Minute)
	s.Equal(ErrNotFound, err)
}

func (s *MemoryCacheTestSuite) TestExpire_NoExpire() {
	s.cache.Set(s.ctx, "key1", "value1", time.Minute)

	err := s.cache.Expire(s.ctx, "key1", 0)
	s.NoError(err)

	// 应该永不过期
	ttl, _ := s.cache.TTL(s.ctx, "key1")
	s.Equal(time.Duration(-1), ttl)
}

func (s *MemoryCacheTestSuite) TestTTL() {
	s.cache.Set(s.ctx, "key1", "value1", time.Minute)

	ttl, err := s.cache.TTL(s.ctx, "key1")
	s.NoError(err)
	s.True(ttl > 0 && ttl <= time.Minute)
}

func (s *MemoryCacheTestSuite) TestTTL_NotFound() {
	ttl, err := s.cache.TTL(s.ctx, "nonexistent")
	s.NoError(err)
	s.Equal(time.Duration(-2), ttl)
}

func (s *MemoryCacheTestSuite) TestTTL_NoExpire() {
	s.cache.Set(s.ctx, "key1", "value1", 0)

	ttl, err := s.cache.TTL(s.ctx, "key1")
	s.NoError(err)
	s.Equal(time.Duration(-1), ttl)
}

func (s *MemoryCacheTestSuite) TestTryLock() {
	ok, err := s.cache.TryLock(s.ctx, "lock1", "owner1", time.Minute)
	s.NoError(err)
	s.True(ok)

	ok, err = s.cache.TryLock(s.ctx, "lock1", "owner2", time.Minute)
	s.NoError(err)
	s.False(ok)
}

func (s *MemoryCacheTestSuite) TestUnlock() {
	s.cache.TryLock(s.ctx, "lock1", "owner1", time.Minute)

	err := s.cache.Unlock(s.ctx, "lock1", "owner1")
	s.NoError(err)

	// 锁已释放，可以再次获取
	ok, _ := s.cache.TryLock(s.ctx, "lock1", "owner2", time.Minute)
	s.True(ok)
}

func (s *MemoryCacheTestSuite) TestUnlock_WrongOwner() {
	s.cache.TryLock(s.ctx, "lock1", "owner1", time.Minute)

	err := s.cache.Unlock(s.ctx, "lock1", "owner2")
	s.Equal(ErrLockNotHeld, err)
}

func (s *MemoryCacheTestSuite) TestUnlock_NotFound() {
	err := s.cache.Unlock(s.ctx, "nonexistent", "owner1")
	s.Equal(ErrLockNotHeld, err)
}

func (s *MemoryCacheTestSuite) TestMGet() {
	s.cache.Set(s.ctx, "key1", "value1", time.Minute)
	s.cache.Set(s.ctx, "key2", "value2", time.Minute)

	values, err := s.cache.MGet(s.ctx, "key1", "key2", "key3")
	s.NoError(err)
	s.Len(values, 3)
	s.Equal("value1", values[0])
	s.Equal("value2", values[1])
	s.Equal("", values[2])
}

func (s *MemoryCacheTestSuite) TestMSet() {
	pairs := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}

	err := s.cache.MSet(s.ctx, pairs, time.Minute)
	s.NoError(err)

	v1, _ := s.cache.Get(s.ctx, "key1")
	v2, _ := s.cache.Get(s.ctx, "key2")
	s.Equal("value1", v1)
	s.Equal("value2", v2)
}

func (s *MemoryCacheTestSuite) TestPing() {
	err := s.cache.Ping(s.ctx)
	s.NoError(err)
}

func (s *MemoryCacheTestSuite) TestClient() {
	client := s.cache.Client()
	// 内存缓存无外部客户端，返回 nil 以避免暴露内部 map
	s.Nil(client)
}

func (s *MemoryCacheTestSuite) TestSize() {
	s.cache.Set(s.ctx, "key1", "value1", time.Minute)
	s.cache.Set(s.ctx, "key2", "value2", time.Minute)

	s.Equal(2, s.cache.Size())
}

func (s *MemoryCacheTestSuite) TestCleanup() {
	s.cache.Set(s.ctx, "key1", "value1", 10*time.Millisecond)
	s.cache.Set(s.ctx, "key2", "value2", time.Hour)

	// 等待清理协程执行
	time.Sleep(200 * time.Millisecond)

	s.Equal(1, s.cache.Size())
}

func (s *MemoryCacheTestSuite) TestEviction() {
	config := &Config{
		Type:    TypeMemory,
		MaxSize: 3,
	}
	config.ApplyDefaults()

	cache, err := NewMemoryCache(config, s.logger)
	s.Require().NoError(err)
	defer cache.Close()

	cache.Set(s.ctx, "key1", "value1", time.Minute)
	cache.Set(s.ctx, "key2", "value2", time.Minute)
	cache.Set(s.ctx, "key3", "value3", time.Minute)
	cache.Set(s.ctx, "key4", "value4", time.Minute)

	// 最多保留 3 个
	mc := cache.(*memoryCache)
	s.LessOrEqual(mc.Size(), 3)
}

func (s *MemoryCacheTestSuite) TestEviction_NoExpiredItems() {
	// 测试 evictOne 在没有过期项时删除第一个找到的项
	config := &Config{
		Type:    TypeMemory,
		MaxSize: 2,
	}
	config.ApplyDefaults()

	cache, err := NewMemoryCache(config, s.logger)
	s.Require().NoError(err)
	defer cache.Close()

	mc := cache.(*memoryCache)

	// 设置两个永不过期的项
	cache.Set(s.ctx, "key1", "value1", 0) // noExpire = true
	cache.Set(s.ctx, "key2", "value2", 0) // noExpire = true

	// 当达到 MaxSize 时添加新项，会触发淘汰非过期项
	cache.Set(s.ctx, "key3", "value3", 0)

	// 应该只有 2 个项
	s.Equal(2, mc.Size())
}

func (s *MemoryCacheTestSuite) TestSet_SerializeError() {
	// 测试序列化错误（使用无法序列化的类型）
	err := s.cache.Set(s.ctx, "key1", make(chan int), time.Minute)
	s.Error(err)
	s.Contains(err.Error(), "序列化值失败")
}

func (s *MemoryCacheTestSuite) TestSetNX_SerializeError() {
	// 测试 SetNX 序列化错误
	ok, err := s.cache.SetNX(s.ctx, "key1", make(chan int), time.Minute)
	s.Error(err)
	s.False(ok)
	s.Contains(err.Error(), "序列化值失败")
}

func (s *MemoryCacheTestSuite) TestSetNX_NoExpire() {
	// 测试 SetNX 永不过期的情况
	ok, err := s.cache.SetNX(s.ctx, "key1", "value1", 0)
	s.NoError(err)
	s.True(ok)

	ttl, _ := s.cache.TTL(s.ctx, "key1")
	s.Equal(time.Duration(-1), ttl)
}

func (s *MemoryCacheTestSuite) TestMSet_SerializeError() {
	// 测试 MSet 序列化错误
	pairs := map[string]any{
		"key1": make(chan int), // 无法序列化
	}

	err := s.cache.MSet(s.ctx, pairs, time.Minute)
	s.Error(err)
	s.Contains(err.Error(), "序列化值失败")
}

func (s *MemoryCacheTestSuite) TestMSet_NoExpire() {
	// 测试 MSet 永不过期
	pairs := map[string]any{
		"key1": "value1",
	}

	err := s.cache.MSet(s.ctx, pairs, 0)
	s.NoError(err)

	ttl, _ := s.cache.TTL(s.ctx, "key1")
	s.Equal(time.Duration(-1), ttl)
}

func (s *MemoryCacheTestSuite) TestTTL_Expired() {
	// 测试 TTL 获取已过期的键（ttl < 0 分支）
	s.cache.Set(s.ctx, "key1", "value1", 5*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	ttl, err := s.cache.TTL(s.ctx, "key1")
	s.NoError(err)
	s.Equal(time.Duration(-2), ttl)
}
