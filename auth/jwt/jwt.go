// Package jwt 提供 JWT 认证功能.
//
// 特性：
//   - 生成、验证、刷新令牌
//   - 可选的缓存集成（用于令牌撤销）
//   - HTTP/gRPC 中间件
//   - 白名单支持
//   - Functional Options 模式
//
// 示例：
//
//	j := jwt.NewJWT(
//	    jwt.WithSecretKey("your-secret-key"),
//	    jwt.WithIssuer("my-service"),
//	    jwt.WithLogger(log),
//	)
//
//	// 生成令牌
//	token, err := j.Generate(claims)
//
//	// 验证令牌
//	claims, err := j.Validate(token)
package jwt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/storage/cache"
)

// TokenStore 令牌存储接口.
//
// 这是 JWT 令牌缓存的最小依赖接口.
// 可以用 cache.Cache、Redis 客户端或其他存储实现.
type TokenStore interface {
	// Get 获取令牌.
	Get(ctx context.Context, key string) (string, error)

	// Set 存储令牌.
	Set(ctx context.Context, key string, value string, ttl time.Duration) error

	// Delete 删除令牌.
	Delete(ctx context.Context, keys ...string) error
}

// TokenStoreWithKeys 支持按模式查询 key 的令牌存储接口.
//
// 如果 TokenStore 实现了此接口，Revoke 会使用 Keys 方法查找并删除匹配的令牌.
// 否则 Revoke 会设置一个撤销标记来使令牌失效.
type TokenStoreWithKeys interface {
	TokenStore
	// Keys 按模式查找 key（支持 * 通配符）.
	Keys(ctx context.Context, pattern string) ([]string, error)
}

// cacheTokenStore 是 cache.Cache 到 TokenStore 的适配器.
type cacheTokenStore struct {
	cache cache.Cache
}

// CacheTokenStore 将 cache.Cache 适配为 TokenStore 接口.
//
// 示例:
//
//	redisCache, _ := cache.New(&cache.Config{Type: "redis", ...})
//	j := jwt.NewJWT(
//	    jwt.WithSecretKey("secret"),
//	    jwt.WithTokenStore(jwt.CacheTokenStore(redisCache)),
//	    jwt.WithLogger(log),
//	)
func CacheTokenStore(c cache.Cache) TokenStore {
	return &cacheTokenStore{cache: c}
}

func (c *cacheTokenStore) Get(ctx context.Context, key string) (string, error) {
	return c.cache.Get(ctx, key)
}

func (c *cacheTokenStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.cache.Set(ctx, key, value, ttl)
}

func (c *cacheTokenStore) Delete(ctx context.Context, keys ...string) error {
	return c.cache.Del(ctx, keys...)
}

// JWT JWT 服务.
type JWT struct {
	opts *options
}

// NewJWT 创建 JWT 服务.
//
// 必须设置 logger，且必须配置以下签名方式之一:
//   - HMAC: 使用 WithSecretKey（对称签名，默认 HS256）
//   - RSA: 使用 WithRSAKeys 或 WithRSAKeyFiles（RS256）
//   - ECDSA: 使用 WithECDSAKeys 或 WithECDSAKeyFiles（ES256）
//   - EdDSA: 使用 WithEdDSAKeys 或 WithEdDSAKeyFiles（Ed25519）
//
// HMAC 模式要求 secretKey 至少 32 字节.
// 非对称模式至少需要 publicKey（验证），privateKey 可选（仅签名时需要）.
func NewJWT(opts ...Option) *JWT {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.logger == nil {
		panic("jwt: 必须设置 logger")
	}

	// 非对称签名模式：验证公钥是否存在
	if o.signingMethod != nil {
		if o.publicKey == nil {
			panic("jwt: 非对称签名模式必须设置公钥（publicKey）")
		}
	} else {
		// HMAC 对称签名模式
		if o.secretKey == "" {
			panic("jwt: 必须设置 secretKey 或非对称密钥对")
		}
		if len(o.secretKey) < 32 {
			panic("jwt: JWT 密钥长度不足，HMAC-SHA256 至少需要 32 字节")
		}
	}

	return &JWT{opts: o}
}

// Generate 生成 JWT 令牌.
//
// 使用配置的签名算法（默认 HMAC-SHA256，也支持 RS256/ES256/EdDSA）.
func (j *JWT) Generate(ctx context.Context, claims Claims) (string, error) {
	token := jwt.NewWithClaims(j.getSigningMethod(), claims)
	tokenString, err := token.SignedString(j.signingKey())
	if err != nil {
		j.opts.logger.With(
			logger.String("name", j.opts.name),
			logger.Err(err),
		).Error("[JWT] 生成令牌失败")
		return "", fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	// 添加前缀
	tokenString = j.opts.tokenPrefix + tokenString

	// 存储到缓存
	if j.opts.store != nil {
		subject, _ := claims.GetSubject()
		exp, err := claims.GetExpirationTime()
		if err == nil && exp != nil {
			iat, _ := claims.GetIssuedAt()
			key := j.buildCacheKey(subject, iat.Unix(), exp.Unix())
			ttl := time.Until(exp.Time)
			if err := j.opts.store.Set(ctx, key, tokenString, ttl); err != nil {
				j.opts.logger.With(
					logger.String("name", j.opts.name),
					logger.String("subject", subject),
					logger.Err(err),
				).Debug("[JWT] 令牌缓存存储失败")
			}
		}
	}

	subject, _ := claims.GetSubject()
	j.opts.logger.With(
		logger.String("name", j.opts.name),
		logger.String("subject", subject),
	).Debug("[JWT] 令牌生成成功")
	return tokenString, nil
}

// GenerateWithDuration 使用指定有效期生成令牌.
func (j *JWT) GenerateWithDuration(claims jwt.Claims, duration time.Duration) (string, error) {
	token := jwt.NewWithClaims(j.getSigningMethod(), claims)
	tokenString, err := token.SignedString(j.signingKey())
	if err != nil {
		j.opts.logger.With(
			logger.String("name", j.opts.name),
			logger.Err(err),
		).Error("[JWT] 生成令牌失败")
		return "", fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	tokenString = j.opts.tokenPrefix + tokenString
	return tokenString, nil
}

// Validate 验证 JWT 令牌.
func (j *JWT) Validate(ctx context.Context, tokenString string) (jwt.Claims, error) {
	return j.ValidateWithClaims(ctx, tokenString, &StandardClaims{})
}

// ValidateWithClaims 使用自定义 Claims 类型验证令牌.
func (j *JWT) ValidateWithClaims(ctx context.Context, tokenString string, claims jwt.Claims) (jwt.Claims, error) {
	// 移除前缀
	tokenString = j.stripPrefix(tokenString)
	if tokenString == "" {
		return nil, ErrTokenEmpty
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// 验证签名算法是否与配置一致，防止 "none" 算法攻击
		if j.opts.signingMethod != nil {
			if token.Method.Alg() != j.opts.signingMethod.Alg() {
				return nil, ErrSigningMethod
			}
			return j.verificationKey(), nil
		}
		// HMAC 模式
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrSigningMethod
		}
		return []byte(j.opts.secretKey), nil
	})

	if err != nil {
		j.opts.logger.With(
			logger.String("name", j.opts.name),
			logger.Err(err),
		).Warn("[JWT] 令牌验证失败")
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	// 验证缓存中的令牌
	if j.opts.store != nil {
		if err := j.validateCachedToken(ctx, tokenString, token.Claims); err != nil {
			return nil, err
		}
	}

	return token.Claims, nil
}

// Refresh 刷新令牌.
func (j *JWT) Refresh(ctx context.Context, tokenString string, newClaims Claims) (string, error) {
	return j.RefreshWithClaims(ctx, tokenString, &StandardClaims{}, newClaims)
}

// RefreshWithClaims 使用自定义 Claims 类型刷新令牌.
func (j *JWT) RefreshWithClaims(ctx context.Context, tokenString string, oldClaimsType jwt.Claims, newClaims Claims) (string, error) {
	// 先尝试正常验证
	_, err := j.ValidateWithClaims(ctx, tokenString, oldClaimsType)
	if err == nil {
		return j.Generate(ctx, newClaims)
	}

	// 如果验证失败，尝试解析过期令牌（仍需验证签名方法，防止 "none" 算法攻击）
	tokenString = j.stripPrefix(tokenString)
	token, parseErr := jwt.ParseWithClaims(tokenString, oldClaimsType, func(token *jwt.Token) (any, error) {
		if j.opts.signingMethod != nil {
			if token.Method.Alg() != j.opts.signingMethod.Alg() {
				return nil, ErrSigningMethod
			}
			return j.verificationKey(), nil
		}
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrSigningMethod
		}
		return []byte(j.opts.secretKey), nil
	})

	if parseErr != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenInvalid, parseErr)
	}

	// 检查是否在刷新窗口内
	exp, err := token.Claims.GetExpirationTime()
	if err != nil || exp == nil {
		return "", ErrClaimsInvalid
	}

	if time.Since(exp.Time) > j.opts.refreshWindow {
		return "", ErrRefreshExpired
	}

	// 生成新令牌
	newToken, err := j.Generate(ctx, newClaims)
	if err != nil {
		return "", err
	}

	newSubject, _ := newClaims.GetSubject()
	j.opts.logger.With(
		logger.String("name", j.opts.name),
		logger.String("subject", newSubject),
	).Debug("[JWT] 令牌刷新成功")
	return newToken, nil
}

// Revoke 撤销用户的所有令牌.
func (j *JWT) Revoke(ctx context.Context, subject string) error {
	if j.opts.store == nil {
		j.opts.logger.With(
			logger.String("name", j.opts.name),
			logger.String("subject", subject),
		).Debug("[JWT] 未配置存储，无需撤销令牌")
		return nil
	}

	pattern := j.opts.cacheKeyPrefix + subject + ":*"

	// 如果存储支持按模式查询 key，则查找并删除匹配的令牌
	if storeWithKeys, ok := j.opts.store.(TokenStoreWithKeys); ok {
		keys, err := storeWithKeys.Keys(ctx, pattern)
		if err != nil {
			j.opts.logger.With(
				logger.String("name", j.opts.name),
				logger.String("pattern", pattern),
				logger.Err(err),
			).Error("[JWT] 查询令牌 key 失败")
			return fmt.Errorf("jwt: 查询令牌 key 失败: %w", err)
		}

		if len(keys) > 0 {
			if err := storeWithKeys.Delete(ctx, keys...); err != nil {
				j.opts.logger.With(
					logger.String("name", j.opts.name),
					logger.String("subject", subject),
					logger.Err(err),
				).Error("[JWT] 删除令牌失败")
				return fmt.Errorf("jwt: 删除令牌失败: %w", err)
			}
		}
	} else {
		// 存储不支持 Keys 查询，设置撤销标记使该用户的令牌在验证时失效
		revokeKey := j.opts.cacheKeyPrefix + "revoked:" + subject
		if err := j.opts.store.Set(ctx, revokeKey, "1", j.opts.accessDuration); err != nil {
			j.opts.logger.With(
				logger.String("name", j.opts.name),
				logger.String("subject", subject),
				logger.Err(err),
			).Error("[JWT] 设置撤销标记失败")
			return fmt.Errorf("jwt: 设置撤销标记失败: %w", err)
		}
	}

	j.opts.logger.With(
		logger.String("name", j.opts.name),
		logger.String("subject", subject),
	).Info("[JWT] 令牌已撤销")

	return nil
}

// IsWhitelisted 检查请求是否在白名单中.
func (j *JWT) IsWhitelisted(ctx context.Context, req any) bool {
	if j.opts.whitelist == nil {
		return false
	}
	return j.opts.whitelist.IsWhitelisted(ctx, req)
}

// ExtractToken 从请求中提取令牌.
func (j *JWT) ExtractToken(ctx context.Context, req any) (string, error) {
	return ExtractToken(ctx, req)
}

// AccessDuration 返回访问令牌有效期.
func (j *JWT) AccessDuration() time.Duration {
	return j.opts.accessDuration
}

// RefreshDuration 返回刷新令牌有效期.
func (j *JWT) RefreshDuration() time.Duration {
	return j.opts.refreshDuration
}

// Issuer 返回签发者.
func (j *JWT) Issuer() string {
	return j.opts.issuer
}

// Name 返回服务名称.
func (j *JWT) Name() string {
	return j.opts.name
}

// stripPrefix 移除令牌前缀.
func (j *JWT) stripPrefix(token string) string {
	token = strings.TrimPrefix(token, j.opts.tokenPrefix)
	return strings.TrimSpace(token)
}

// buildCacheKey 构建缓存 key.
func (j *JWT) buildCacheKey(subject string, iat int64, exp int64) string {
	return fmt.Sprintf("%s%s:%d:%d", j.opts.cacheKeyPrefix, subject, iat, exp)
}

// getSigningMethod 获取签名算法.
//
// 非对称模式返回配置的算法，HMAC 模式返回 HS256.
func (j *JWT) getSigningMethod() jwt.SigningMethod {
	if j.opts.signingMethod != nil {
		return j.opts.signingMethod
	}
	return jwt.SigningMethodHS256
}

// signingKey 获取签名密钥.
//
// 非对称模式返回私钥，HMAC 模式返回 secretKey 字节.
func (j *JWT) signingKey() any {
	if j.opts.signingMethod != nil {
		return j.opts.privateKey
	}
	return []byte(j.opts.secretKey)
}

// verificationKey 获取验证密钥.
//
// 非对称模式返回公钥，HMAC 模式返回 secretKey 字节.
func (j *JWT) verificationKey() any {
	if j.opts.signingMethod != nil {
		return j.opts.publicKey
	}
	return []byte(j.opts.secretKey)
}

// validateCachedToken 验证缓存中的令牌.
func (j *JWT) validateCachedToken(ctx context.Context, tokenString string, claims jwt.Claims) error {
	iat, err := claims.GetIssuedAt()
	if err != nil || iat == nil {
		return ErrClaimsInvalid
	}

	subject, err := claims.GetSubject()
	if err != nil {
		return ErrClaimsInvalid
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return ErrClaimsInvalid
	}

	// 检查撤销标记（用于不支持 Keys 查询的存储）
	revokeKey := j.opts.cacheKeyPrefix + "revoked:" + subject
	if val, revokeErr := j.opts.store.Get(ctx, revokeKey); revokeErr == nil && val != "" {
		return ErrTokenRevoked
	} else if revokeErr != nil && !errors.Is(revokeErr, cache.ErrNotFound) {
		// 缓存访问错误（如 Redis 宕机），跳过撤销检查，fail-open
		j.opts.logger.With(
			logger.String("name", j.opts.name),
			logger.String("subject", subject),
			logger.Err(revokeErr),
		).Warn("[JWT] 缓存撤销标记查询失败，跳过检查")
	}

	key := j.buildCacheKey(subject, iat.Unix(), exp.Unix())
	storedToken, err := j.opts.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			// 缓存中无此令牌，视为已撤销
			return ErrTokenRevoked
		}
		// 缓存访问错误（如 Redis 宕机），fail-open，跳过缓存校验
		j.opts.logger.With(
			logger.String("name", j.opts.name),
			logger.String("subject", subject),
			logger.Err(err),
		).Warn("[JWT] 缓存令牌查询失败，跳过缓存校验")
		return nil
	}

	storedToken = j.stripPrefix(storedToken)
	if storedToken != tokenString {
		return ErrTokenRevoked
	}

	return nil
}
