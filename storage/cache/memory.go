package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// memoryCache 内存缓存实现.
type memoryCache struct {
	data    map[string]*cacheItem
	mu      sync.RWMutex
	config  *Config
	logger  logger.Logger
	closeCh chan struct{}
}

// cacheItem 缓存项.
type cacheItem struct {
	value        string
	expireAt     time.Time
	noExpire     bool
	lastAccessed time.Time
}

// isExpired 检查是否过期.
func (i *cacheItem) isExpired() bool {
	if i.noExpire {
		return false
	}
	return time.Now().After(i.expireAt)
}

// NewMemoryCache 创建内存缓存.
func NewMemoryCache(config *Config, log logger.Logger) (Cache, error) {
	if config == nil {
		config = NewMemoryConfig()
	}
	log = logger.WithComponent(log, "Cache")
	config.ApplyDefaults()

	c := &memoryCache{
		data:    make(map[string]*cacheItem),
		config:  config,
		logger:  log,
		closeCh: make(chan struct{}),
	}

	// 启动清理协程
	go c.cleanupLoop()

	log.Debug("内存缓存初始化完成")

	return c, nil
}

// cleanupLoop 定期清理过期项.
func (m *memoryCache) cleanupLoop() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.closeCh:
			return
		}
	}
}

// cleanup 清理过期项.
func (m *memoryCache) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, item := range m.data {
		if item.isExpired() {
			delete(m.data, key)
		}
	}
}

// Set 设置键值对.
func (m *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := m.serialize(value)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查容量
	if len(m.data) >= m.config.MaxSize {
		// 简单策略：删除一个过期的或第一个
		m.evictOne()
	}

	now := time.Now()
	item := &cacheItem{value: data, lastAccessed: now}
	if ttl > 0 {
		item.expireAt = now.Add(ttl)
	} else {
		item.noExpire = true
	}

	m.data[key] = item
	return nil
}

// evictOne 淘汰一个缓存项.
func (m *memoryCache) evictOne() {
	// 优先删除过期项
	for key, item := range m.data {
		if item.isExpired() {
			delete(m.data, key)
			return
		}
	}

	// 淘汰最久未访问的项LRU
	var oldestKey string
	var oldestTime time.Time
	first := true
	for key, item := range m.data {
		if first || item.lastAccessed.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.lastAccessed
			first = false
		}
	}
	if !first {
		delete(m.data, oldestKey)
	}
}

// Get 获取值.
func (m *memoryCache) Get(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	item, ok := m.data[key]
	if !ok {
		m.mu.Unlock()
		return "", ErrNotFound
	}

	if item.isExpired() {
		delete(m.data, key)
		m.mu.Unlock()
		return "", ErrNotFound
	}

	item.lastAccessed = time.Now()
	m.mu.Unlock()

	return item.value, nil
}

// Del 删除键.
func (m *memoryCache) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Exists 检查键是否存在.
// 注意：在单次加锁内完成查找和过期清理，避免 TOCTOU 竞态.
func (m *memoryCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.data[key]
	if !ok {
		return false, nil
	}

	if item.isExpired() {
		delete(m.data, key)
		return false, nil
	}

	return true, nil
}

// SetNX 仅当键不存在时设置.
func (m *memoryCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item, ok := m.data[key]; ok && !item.isExpired() {
		return false, nil
	}

	data, err := m.serialize(value)
	if err != nil {
		return false, err
	}

	now := time.Now()
	item := &cacheItem{value: data, lastAccessed: now}
	if ttl > 0 {
		item.expireAt = now.Add(ttl)
	} else {
		item.noExpire = true
	}

	m.data[key] = item
	return true, nil
}

// Increment 递增.
func (m *memoryCache) Increment(ctx context.Context, key string) (int64, error) {
	return m.IncrementBy(ctx, key, 1)
}

// IncrementBy 增加指定值.
func (m *memoryCache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var current int64

	if item, ok := m.data[key]; ok && !item.isExpired() {
		if _, err := fmt.Sscanf(item.value, "%d", &current); err != nil {
			return 0, ErrNotInteger
		}
	}

	current += value
	m.data[key] = &cacheItem{
		value:        fmt.Sprintf("%d", current),
		noExpire:     true,
		lastAccessed: time.Now(),
	}

	return current, nil
}

// Decrement 递减.
func (m *memoryCache) Decrement(ctx context.Context, key string) (int64, error) {
	return m.IncrementBy(ctx, key, -1)
}

// Expire 设置过期时间.
func (m *memoryCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.data[key]
	if !ok {
		return ErrNotFound
	}

	if ttl > 0 {
		item.expireAt = time.Now().Add(ttl)
		item.noExpire = false
	} else {
		item.noExpire = true
	}

	return nil
}

// TTL 获取剩余过期时间.
func (m *memoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	item, ok := m.data[key]
	m.mu.RUnlock()

	if !ok {
		return -2, nil // Redis 约定：-2 表示键不存在
	}

	if item.noExpire {
		return -1, nil // Redis 约定：-1 表示永不过期
	}

	ttl := time.Until(item.expireAt)
	if ttl < 0 {
		return -2, nil
	}

	return ttl, nil
}

// TryLock 尝试获取锁.
func (m *memoryCache) TryLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return m.SetNX(ctx, key, value, ttl)
}

// ExtendLock 原子地验证锁持有者并延长过期时间.
func (m *memoryCache) ExtendLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.data[key]
	if !ok || item.isExpired() {
		return false, nil
	}

	if item.value != value {
		return false, nil
	}

	if ttl > 0 {
		item.expireAt = time.Now().Add(ttl)
		item.noExpire = false
	} else {
		item.noExpire = true
	}

	return true, nil
}

// Unlock 释放锁.
func (m *memoryCache) Unlock(ctx context.Context, key string, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.data[key]
	if !ok {
		return ErrLockNotHeld
	}

	if item.value != value {
		return ErrLockNotHeld
	}

	delete(m.data, key)
	return nil
}

// MGet 批量获取.
func (m *memoryCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	values := make([]string, len(keys))
	for i, key := range keys {
		if item, ok := m.data[key]; ok && !item.isExpired() {
			values[i] = item.value
		}
	}
	return values, nil
}

// MSet 批量设置.
func (m *memoryCache) MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, value := range pairs {
		data, err := m.serialize(value)
		if err != nil {
			return err
		}

		item := &cacheItem{value: data, lastAccessed: now}
		if ttl > 0 {
			item.expireAt = now.Add(ttl)
		} else {
			item.noExpire = true
		}
		m.data[key] = item
	}
	return nil
}

// Ping 测试连接内存缓存始终可用.
func (m *memoryCache) Ping(ctx context.Context) error {
	return nil
}

// Close 关闭缓存.
func (m *memoryCache) Close() error {
	close(m.closeCh)
	m.logger.Debug("内存缓存已关闭")
	return nil
}

// Client 返回底层客户端.
// 内存缓存无外部客户端，返回 nil 以避免暴露内部 map 绕过锁保护.
func (m *memoryCache) Client() any {
	return nil
}

// serialize 序列化值.
func (m *memoryCache) serialize(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return "", ErrSerialize
		}
		return string(data), nil
	}
}

// Size 返回缓存大小仅用于测试.
func (m *memoryCache) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}
