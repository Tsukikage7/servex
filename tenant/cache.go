package tenant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// CacheStore 缓存存储接口.
type CacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// CacheOption 缓存配置选项.
type CacheOption func(*cacheOptions)

type cacheOptions struct {
	ttl       time.Duration
	prefix    string
	marshal   func(Tenant) (string, error)
	unmarshal func(string) (Tenant, error)
}

func defaultCacheOptions() *cacheOptions {
	return &cacheOptions{
		ttl:    5 * time.Minute,
		prefix: "tenant:",
	}
}

// WithCacheTTL 设置缓存过期时间（默认 5 分钟）.
func WithCacheTTL(ttl time.Duration) CacheOption {
	return func(o *cacheOptions) { o.ttl = ttl }
}

// WithCachePrefix 设置缓存键前缀（默认 "tenant:"）.
func WithCachePrefix(prefix string) CacheOption {
	return func(o *cacheOptions) { o.prefix = prefix }
}

// WithMarshal 设置租户序列化函数.
func WithMarshal(fn func(Tenant) (string, error)) CacheOption {
	return func(o *cacheOptions) { o.marshal = fn }
}

// WithUnmarshal 设置租户反序列化函数.
func WithUnmarshal(fn func(string) (Tenant, error)) CacheOption {
	return func(o *cacheOptions) { o.unmarshal = fn }
}

// cachedResolver 带缓存的 Resolver 包装.
type cachedResolver struct {
	inner Resolver
	store CacheStore
	opts  *cacheOptions
}

// NewCachedResolver 创建带缓存的 Resolver.
// 用户必须提供 WithMarshal 和 WithUnmarshal 回调，因为库不知道具体租户类型.
// 示例:
//
//	cached := tenant.NewCachedResolver(dbResolver, redisStore,
//	    tenant.WithCacheTTL(10*time.Minute),
//	    tenant.WithMarshal(func(t tenant.Tenant) (string, error) {
//	        return json.Marshal(t.(*MyTenant))
//	    }),
//	    tenant.WithUnmarshal(func(s string) (tenant.Tenant, error) {
//	        var t MyTenant
//	        err := json.Unmarshal([]byte(s), &t)
//	        return &t, err
//	    }),
//	)
func NewCachedResolver(inner Resolver, store CacheStore, opts ...CacheOption) Resolver {
	if inner == nil {
		panic("tenant: 内部解析器不能为空")
	}
	if store == nil {
		panic("tenant: 缓存存储不能为空")
	}

	o := defaultCacheOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.marshal == nil || o.unmarshal == nil {
		panic("tenant: 必须提供 WithMarshal 和 WithUnmarshal")
	}

	return &cachedResolver{inner: inner, store: store, opts: o}
}

func (r *cachedResolver) Resolve(ctx context.Context, token string) (Tenant, error) {
	// 使用 token 的 hash 作为缓存 key，避免在缓存中存储原始 token 值
	h := sha256.Sum256([]byte(token))
	key := r.opts.prefix + hex.EncodeToString(h[:])

	// 尝试从缓存获取
	val, err := r.store.Get(ctx, key)
	if err == nil && val != "" {
		t, err := r.opts.unmarshal(val)
		if err == nil {
			return t, nil
		}
		// 反序列化失败，回退到内部解析器
	}

	// 缓存未命中，调用内部解析器
	t, err := r.inner.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}

	// 写入缓存（忽略错误）
	if val, err := r.opts.marshal(t); err == nil {
		_ = r.store.Set(ctx, key, val, r.opts.ttl)
	}

	return t, nil
}

// 编译期接口断言.
var _ Resolver = (*cachedResolver)(nil)
