// Package captcha 提供验证码生命周期管理，包括生成、验证和防刷控制.
package captcha

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCodeExpired 验证码已过期.
	ErrCodeExpired = errors.New("captcha: code expired")
	// ErrCodeInvalid 验证码无效.
	ErrCodeInvalid = errors.New("captcha: invalid code")
	// ErrTooManyAttempts 验证次数过多.
	ErrTooManyAttempts = errors.New("captcha: too many attempts")
	// ErrCooldown 请求过于频繁.
	ErrCooldown = errors.New("captcha: please wait before requesting a new code")
)

// Code 验证码.
type Code struct {
	Key       string    // 验证码标识（如手机号、邮箱）
	Code      string    // 验证码内容
	ExpiresAt time.Time // 过期时间
}

// Manager 验证码管理器.
type Manager interface {
	Generate(ctx context.Context, key string) (*Code, error)
	Verify(ctx context.Context, key string, code string) error
	Invalidate(ctx context.Context, key string) error
}

// Option 配置选项.
type Option func(*options)

type options struct {
	length      int
	expiration  time.Duration
	maxAttempts int
	cooldown    time.Duration
	alphabet    string
}

// WithLength 设置验证码长度，默认 6.
func WithLength(n int) Option {
	return func(o *options) {
		o.length = n
	}
}

// WithExpiration 设置过期时间，默认 5m.
func WithExpiration(d time.Duration) Option {
	return func(o *options) {
		o.expiration = d
	}
}

// WithMaxAttempts 设置最大验证次数，默认 5（防暴力破解）.
func WithMaxAttempts(n int) Option {
	return func(o *options) {
		o.maxAttempts = n
	}
}

// WithCooldown 设置发送冷却时间，默认 60s（防刷）.
func WithCooldown(d time.Duration) Option {
	return func(o *options) {
		o.cooldown = d
	}
}

// WithAlphabet 设置验证码字符集，默认纯数字.
func WithAlphabet(alphabet string) Option {
	return func(o *options) {
		o.alphabet = alphabet
	}
}

// Store 验证码存储接口.
type Store interface {
	Save(ctx context.Context, key, code string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	IncrAttempts(ctx context.Context, key string) (int, error)
	GetCooldown(ctx context.Context, key string) (time.Duration, error)
	SetCooldown(ctx context.Context, key string, ttl time.Duration) error
}

// mgr 验证码管理器实现.
type mgr struct {
	store Store
	opts  options
}

// NewManager 创建验证码管理器.
func NewManager(store Store, opts ...Option) Manager {
	o := options{
		length:      6,
		expiration:  5 * time.Minute,
		maxAttempts: 5,
		cooldown:    60 * time.Second,
		alphabet:    "0123456789",
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &mgr{store: store, opts: o}
}

// Generate 生成验证码.
func (m *mgr) Generate(ctx context.Context, key string) (*Code, error) {
	// 检查冷却时间
	remaining, err := m.store.GetCooldown(ctx, key)
	if err == nil && remaining > 0 {
		return nil, ErrCooldown
	}

	// 生成随机验证码
	code, err := m.generateCode()
	if err != nil {
		return nil, err
	}

	// 保存验证码
	if err := m.store.Save(ctx, key, code, m.opts.expiration); err != nil {
		return nil, err
	}

	// 设置冷却时间
	_ = m.store.SetCooldown(ctx, key, m.opts.cooldown)

	return &Code{
		Key:       key,
		Code:      code,
		ExpiresAt: time.Now().Add(m.opts.expiration),
	}, nil
}

// Verify 验证验证码.
func (m *mgr) Verify(ctx context.Context, key string, code string) error {
	// 检查尝试次数
	attempts, err := m.store.IncrAttempts(ctx, key)
	if err == nil && attempts > m.opts.maxAttempts {
		// 超过最大尝试次数，删除验证码
		_ = m.store.Delete(ctx, key)
		return ErrTooManyAttempts
	}

	// 获取存储的验证码
	stored, err := m.store.Get(ctx, key)
	if err != nil {
		return ErrCodeExpired
	}

	if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		return ErrCodeInvalid
	}

	// 验证成功，删除验证码
	_ = m.store.Delete(ctx, key)
	return nil
}

// Invalidate 使验证码失效.
func (m *mgr) Invalidate(ctx context.Context, key string) error {
	return m.store.Delete(ctx, key)
}

// generateCode 生成随机验证码.
func (m *mgr) generateCode() (string, error) {
	result := make([]byte, m.opts.length)
	alphabetLen := big.NewInt(int64(len(m.opts.alphabet)))
	for i := range result {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = m.opts.alphabet[n.Int64()]
	}
	return string(result), nil
}

// --- Memory Store ---

type memoryEntry struct {
	code      string
	expiresAt time.Time
}

type memoryStore struct {
	mu        sync.RWMutex
	codes     map[string]memoryEntry
	attempts  map[string]int
	cooldowns map[string]time.Time
}

// NewMemoryStore 创建基于内存的验证码存储（用于测试）.
func NewMemoryStore() Store {
	return &memoryStore{
		codes:     make(map[string]memoryEntry),
		attempts:  make(map[string]int),
		cooldowns: make(map[string]time.Time),
	}
}

func (s *memoryStore) Save(_ context.Context, key, code string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[key] = memoryEntry{code: code, expiresAt: time.Now().Add(ttl)}
	s.attempts[key] = 0 // 重置尝试次数
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.codes[key]
	if !ok {
		return "", ErrCodeExpired
	}
	if time.Now().After(entry.expiresAt) {
		return "", ErrCodeExpired
	}
	return entry.code, nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.codes, key)
	delete(s.attempts, key)
	delete(s.cooldowns, key) // 同时清理冷却记录
	return nil
}

func (s *memoryStore) IncrAttempts(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[key]++
	return s.attempts[key], nil
}

func (s *memoryStore) GetCooldown(_ context.Context, key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expiresAt, ok := s.cooldowns[key]
	if !ok {
		return 0, nil
	}
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return 0, nil
	}
	return remaining, nil
}

func (s *memoryStore) SetCooldown(_ context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cooldowns[key] = time.Now().Add(ttl)
	return nil
}

// --- Redis Store ---

type redisStore struct {
	client redis.Cmdable
}

// NewRedisStore 创建基于 Redis 的验证码存储.
func NewRedisStore(client redis.Cmdable) Store {
	return &redisStore{client: client}
}

func (s *redisStore) codeKey(key string) string {
	return "captcha:code:" + key
}

func (s *redisStore) attemptKey(key string) string {
	return "captcha:attempt:" + key
}

func (s *redisStore) cooldownKey(key string) string {
	return "captcha:cooldown:" + key
}

func (s *redisStore) Save(ctx context.Context, key, code string, ttl time.Duration) error {
	return s.client.Set(ctx, s.codeKey(key), code, ttl).Err()
}

func (s *redisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, s.codeKey(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCodeExpired
	}
	return val, err
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.codeKey(key), s.attemptKey(key)).Err()
}

func (s *redisStore) IncrAttempts(ctx context.Context, key string) (int, error) {
	val, err := s.client.Incr(ctx, s.attemptKey(key)).Result()
	if err != nil {
		return 0, err
	}
	// 设置过期时间与验证码一致
	s.client.Expire(ctx, s.attemptKey(key), 10*time.Minute)
	return int(val), nil
}

func (s *redisStore) GetCooldown(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := s.client.TTL(ctx, s.cooldownKey(key)).Result()
	if err != nil {
		return 0, err
	}
	if ttl <= 0 {
		return 0, nil
	}
	return ttl, nil
}

func (s *redisStore) SetCooldown(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Set(ctx, s.cooldownKey(key), "1", ttl).Err()
}
