// Package apikey 提供 AI 服务的 API Key 管理功能.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/auth"
	authapikey "github.com/Tsukikage7/servex/v2/auth/apikey"
)

// Key API 密钥模型.
type Key struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name"`
	HashedKey   string     `json:"-" gorm:"uniqueIndex"`
	Prefix      string     `json:"prefix"`
	OwnerID     string     `json:"owner_id" gorm:"index"`
	Permissions []string   `json:"permissions,omitzero" gorm:"serializer:json"`
	RateLimit   int        `json:"rate_limit"`
	QuotaLimit  int64      `json:"quota_limit"`
	QuotaUsed   int64      `json:"quota_used"`
	ExpiresAt   *time.Time `json:"expires_at,omitzero"`
	Enabled     bool       `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	LastUsedAt  *time.Time `json:"last_used_at,omitzero"`
}

// TableName 返回 GORM 表名.
func (Key) TableName() string { return "api_keys" }

// 预定义错误.
var (
	// ErrNilStore Store 为 nil 时返回.
	ErrNilStore = errors.New("apikey: store is nil")
	// ErrKeyNotFound 密钥未找到.
	ErrKeyNotFound = errors.New("apikey: key not found")
	// ErrKeyDisabled 密钥已禁用.
	ErrKeyDisabled = errors.New("apikey: key is disabled")
	// ErrKeyExpired 密钥已过期.
	ErrKeyExpired = errors.New("apikey: key has expired")
	// ErrQuotaExceeded 配额已用尽.
	ErrQuotaExceeded = errors.New("apikey: quota exceeded")
	// ErrRateLimited 请求被限流.
	ErrRateLimited = errors.New("apikey: rate limited")
	// ErrMissingKey 请求中缺少 API 密钥.
	ErrMissingKey = errors.New("apikey: missing API key")
	// ErrInvalidKey 无效的 API 密钥.
	ErrInvalidKey = errors.New("apikey: invalid API key")
)

// Manager API Key 管理器接口.
type Manager interface {
	// Create 创建新的 API Key，返回原始密钥和 Key 对象.
	Create(ctx context.Context, opts ...CreateOption) (rawKey string, key *Key, err error)
	// Validate 验证原始密钥，返回对应的 Key 对象.
	Validate(ctx context.Context, rawKey string) (*Key, error)
	// Revoke 撤销指定 ID 的 API Key.
	Revoke(ctx context.Context, keyID string) error
	// List 列出指定 Owner 的所有 API Key.
	List(ctx context.Context, ownerID string) ([]*Key, error)
	// UpdateQuota 更新指定 Key 的配额使用量.
	UpdateQuota(ctx context.Context, keyID string, tokensUsed int64) error
	// AsValidator 将 Manager 适配为 auth/apikey.Validator，
	// 使其可与 auth/apikey.Authenticator 配合使用.
	AsValidator() authapikey.Validator
}

// RateLimiter 限流接口.
type RateLimiter interface {
	// Allow 判断指定 Key 是否允许请求.
	Allow(ctx context.Context, key string, limit int) (bool, error)
}

// CreateOption 创建 API Key 的选项.
type CreateOption func(*createOptions)

type createOptions struct {
	name        string
	ownerID     string
	permissions []string
	rateLimit   int
	quotaLimit  int64
	expiresAt   *time.Time
}

// WithName 设置 Key 名称.
func WithName(name string) CreateOption {
	return func(o *createOptions) { o.name = name }
}

// WithOwnerID 设置 Key 所有者 ID.
func WithOwnerID(ownerID string) CreateOption {
	return func(o *createOptions) { o.ownerID = ownerID }
}

// WithPermissions 设置 Key 权限列表.
func WithPermissions(permissions []string) CreateOption {
	return func(o *createOptions) { o.permissions = permissions }
}

// WithRateLimit 设置 Key 限流值（每分钟请求数）.
func WithRateLimit(limit int) CreateOption {
	return func(o *createOptions) { o.rateLimit = limit }
}

// WithQuotaLimit 设置 Key 配额上限.
func WithQuotaLimit(limit int64) CreateOption {
	return func(o *createOptions) { o.quotaLimit = limit }
}

// WithExpiresAt 设置 Key 过期时间.
func WithExpiresAt(t time.Time) CreateOption {
	return func(o *createOptions) { o.expiresAt = &t }
}

// ManagerOption Manager 构造选项.
type ManagerOption func(*manager)

// WithRateLimiter 设置限流器.
func WithRateLimiter(rl RateLimiter) ManagerOption {
	return func(m *manager) { m.rateLimiter = rl }
}

// WithKeyPrefix 设置生成密钥的前缀，默认 "sk-".
func WithKeyPrefix(prefix string) ManagerOption {
	return func(m *manager) { m.keyPrefix = prefix }
}

// manager Manager 接口的默认实现.
type manager struct {
	store       Store
	rateLimiter RateLimiter
	keyPrefix   string
	quotaMu     sync.Mutex // 保护 UpdateQuota 的读-改-写操作原子性
}

// NewManager 创建 Manager 实例.
func NewManager(store Store, opts ...ManagerOption) (Manager, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	m := &manager{
		store:     store,
		keyPrefix: "sk-",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func (m *manager) Create(ctx context.Context, opts ...CreateOption) (string, *Key, error) {
	var o createOptions
	for _, opt := range opts {
		opt(&o)
	}

	// 生成 32 字节随机数据
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	rawKey := m.keyPrefix + hex.EncodeToString(raw)

	// SHA-256 哈希
	hashed := hashKey(rawKey)

	key := &Key{
		ID:          uuid.New().String(),
		Name:        o.name,
		HashedKey:   hashed,
		Prefix:      rawKey[:len(m.keyPrefix)+8], // 前缀 + 前 8 个十六进制字符
		OwnerID:     o.ownerID,
		Permissions: o.permissions,
		RateLimit:   o.rateLimit,
		QuotaLimit:  o.quotaLimit,
		ExpiresAt:   o.expiresAt,
		Enabled:     true,
	}

	if err := m.store.Save(ctx, key); err != nil {
		return "", nil, err
	}
	return rawKey, key, nil
}

func (m *manager) Validate(ctx context.Context, rawKey string) (*Key, error) {
	hashed := hashKey(rawKey)

	key, err := m.store.GetByHash(ctx, hashed)
	if err != nil {
		return nil, ErrInvalidKey
	}

	// 检查是否启用
	if !key.Enabled {
		return nil, ErrKeyDisabled
	}

	// 检查是否过期
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, ErrKeyExpired
	}

	// 检查配额
	if key.QuotaLimit > 0 && key.QuotaUsed >= key.QuotaLimit {
		return nil, ErrQuotaExceeded
	}

	// 检查限流
	if m.rateLimiter != nil && key.RateLimit > 0 {
		allowed, rlErr := m.rateLimiter.Allow(ctx, key.ID, key.RateLimit)
		if rlErr != nil {
			return nil, rlErr
		}
		if !allowed {
			return nil, ErrRateLimited
		}
	}

	// 更新最后使用时间
	now := time.Now()
	key.LastUsedAt = &now
	_ = m.store.Update(ctx, key)

	return key, nil
}

func (m *manager) Revoke(ctx context.Context, keyID string) error {
	key, err := m.store.GetByID(ctx, keyID)
	if err != nil {
		return ErrKeyNotFound
	}
	key.Enabled = false
	return m.store.Update(ctx, key)
}

func (m *manager) List(ctx context.Context, ownerID string) ([]*Key, error) {
	return m.store.List(ctx, ownerID)
}

func (m *manager) UpdateQuota(ctx context.Context, keyID string, tokensUsed int64) error {
	// 加锁保证读-改-写操作的原子性，防止并发更新丢失.
	m.quotaMu.Lock()
	defer m.quotaMu.Unlock()

	key, err := m.store.GetByID(ctx, keyID)
	if err != nil {
		return ErrKeyNotFound
	}
	key.QuotaUsed += tokensUsed
	return m.store.Update(ctx, key)
}

// AsValidator 将 Manager 适配为 auth/apikey.Validator，
// 使其可直接用作 authapikey.New(manager.AsValidator()) 的参数.
// 验证通过时返回包含 Key 信息的 auth.Principal.
func (m *manager) AsValidator() authapikey.Validator {
	return func(ctx context.Context, rawKey string) (*auth.Principal, error) {
		key, err := m.Validate(ctx, rawKey)
		if err != nil {
			return nil, err
		}
		p := &auth.Principal{
			ID:          key.OwnerID,
			Type:        auth.PrincipalTypeService,
			Name:        key.Name,
			Permissions: key.Permissions,
			Metadata:    map[string]any{"key_id": key.ID},
			ExpiresAt:   key.ExpiresAt,
		}
		return p, nil
	}
}

// hashKey 使用 SHA-256 对原始密钥进行哈希.
func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// --- Context helpers ---

type contextKey struct{}

// FromContext 从 context 中获取 API Key.
func FromContext(ctx context.Context) (*Key, bool) {
	key, ok := ctx.Value(contextKey{}).(*Key)
	return key, ok
}

// NewContext 将 API Key 注入 context.
func NewContext(ctx context.Context, key *Key) context.Context {
	return context.WithValue(ctx, contextKey{}, key)
}

// --- HTTP 中间件 ---

// HTTPMiddleware 返回验证 API Key 的 HTTP 中间件.
// 从 "Authorization: Bearer sk-xxx" 或 "X-API-Key: sk-xxx" 头部提取密钥.
func HTTPMiddleware(mgr Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := extractKey(r)
			if rawKey == "" {
				http.Error(w, ErrMissingKey.Error(), http.StatusUnauthorized)
				return
			}

			key, err := mgr.Validate(r.Context(), rawKey)
			if err != nil {
				code := http.StatusUnauthorized
				switch {
				case errors.Is(err, ErrRateLimited):
					code = http.StatusTooManyRequests
				case errors.Is(err, ErrQuotaExceeded):
					code = http.StatusForbidden
				}
				http.Error(w, "认证失败", code)
				return
			}

			next.ServeHTTP(w, r.WithContext(NewContext(r.Context(), key)))
		})
	}
}

// extractKey 从 HTTP 请求中提取 API Key.
func extractKey(r *http.Request) string {
	// 优先从 Authorization 头部提取
	if auth := r.Header.Get("Authorization"); auth != "" {
		if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
	}
	// 其次从 X-API-Key 头部提取
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	return ""
}
