package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Tsukikage7/servex/observability/logger"
)

// Option JWT 配置选项.
type Option func(*options)

// options JWT 内部配置.
type options struct {
	name            string
	secretKey       string
	issuer          string
	accessDuration  time.Duration
	refreshDuration time.Duration
	refreshWindow   time.Duration
	tokenPrefix     string
	cacheKeyPrefix  string
	store           TokenStore
	logger          logger.Logger
	whitelist       *Whitelist

	// 非对称签名相关字段
	signingMethod jwt.SigningMethod // 签名算法（nil 表示使用默认的 HS256）
	privateKey    crypto.PrivateKey // 签名私钥（用于生成令牌）
	publicKey     crypto.PublicKey  // 验证公钥（用于验证令牌）
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		name:            "JWT",
		accessDuration:  2 * time.Hour,
		refreshDuration: 7 * 24 * time.Hour,
		refreshWindow:   1 * time.Hour,
		tokenPrefix:     "Bearer ",
		cacheKeyPrefix:  "jwt:token:",
	}
}

// WithName 设置 JWT 服务名称.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithSecretKey 设置签名密钥.
func WithSecretKey(key string) Option {
	return func(o *options) {
		o.secretKey = key
	}
}

// WithIssuer 设置签发者.
func WithIssuer(issuer string) Option {
	return func(o *options) {
		o.issuer = issuer
	}
}

// WithAccessDuration 设置访问令牌有效期.
//
// 默认: 2 小时.
func WithAccessDuration(d time.Duration) Option {
	return func(o *options) {
		o.accessDuration = d
	}
}

// WithRefreshDuration 设置刷新令牌有效期.
//
// 默认: 7 天.
func WithRefreshDuration(d time.Duration) Option {
	return func(o *options) {
		o.refreshDuration = d
	}
}

// WithRefreshWindow 设置过期后可刷新窗口.
//
// 默认: 1 小时.
func WithRefreshWindow(d time.Duration) Option {
	return func(o *options) {
		o.refreshWindow = d
	}
}

// WithTokenPrefix 设置令牌前缀.
//
// 默认: "Bearer ".
func WithTokenPrefix(prefix string) Option {
	return func(o *options) {
		o.tokenPrefix = prefix
	}
}

// WithCacheKeyPrefix 设置缓存 key 前缀.
//
// 默认: "jwt:token:".
func WithCacheKeyPrefix(prefix string) Option {
	return func(o *options) {
		o.cacheKeyPrefix = prefix
	}
}

// WithTokenStore 设置令牌存储（用于令牌存储和撤销）.
//
// 可以使用 CacheTokenStore 适配 cache.Cache:
//
//	jwt.WithTokenStore(jwt.CacheTokenStore(redisCache))
func WithTokenStore(s TokenStore) Option {
	return func(o *options) {
		o.store = s
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

// WithWhitelist 设置白名单.
func WithWhitelist(w *Whitelist) Option {
	return func(o *options) {
		o.whitelist = w
	}
}

// WithRSAKeys 使用 RSA 密钥对进行签名和验证（RS256）.
//
// privateKey 用于签名（可为 nil，仅验证时），publicKey 用于验证.
func WithRSAKeys(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) Option {
	return func(o *options) {
		o.signingMethod = jwt.SigningMethodRS256
		if privateKey != nil {
			o.privateKey = privateKey
		}
		if publicKey != nil {
			o.publicKey = publicKey
		}
	}
}

// WithECDSAKeys 使用 ECDSA 密钥对进行签名和验证（ES256）.
//
// privateKey 用于签名（可为 nil，仅验证时），publicKey 用于验证.
func WithECDSAKeys(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) Option {
	return func(o *options) {
		o.signingMethod = jwt.SigningMethodES256
		if privateKey != nil {
			o.privateKey = privateKey
		}
		if publicKey != nil {
			o.publicKey = publicKey
		}
	}
}

// WithEdDSAKeys 使用 Ed25519 密钥对进行签名和验证（EdDSA）.
//
// privateKey 用于签名（可为 nil，仅验证时），publicKey 用于验证.
func WithEdDSAKeys(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey) Option {
	return func(o *options) {
		o.signingMethod = jwt.SigningMethodEdDSA
		if privateKey != nil {
			o.privateKey = privateKey
		}
		if publicKey != nil {
			o.publicKey = publicKey
		}
	}
}

// WithRSAKeyFiles 从 PEM 文件加载 RSA 密钥对（RS256）.
//
// 如果 privateKeyPath 为空，则只加载公钥（仅验证模式）.
func WithRSAKeyFiles(privateKeyPath, publicKeyPath string) Option {
	return func(o *options) {
		o.signingMethod = jwt.SigningMethodRS256
		if privateKeyPath != "" {
			privKey, err := LoadRSAPrivateKeyFile(privateKeyPath)
			if err != nil {
				panic("jwt: 加载 RSA 私钥文件失败: " + err.Error())
			}
			o.privateKey = privKey
		}
		pubKey, err := LoadRSAPublicKeyFile(publicKeyPath)
		if err != nil {
			panic("jwt: 加载 RSA 公钥文件失败: " + err.Error())
		}
		o.publicKey = pubKey
	}
}

// WithECDSAKeyFiles 从 PEM 文件加载 ECDSA 密钥对（ES256）.
//
// 如果 privateKeyPath 为空，则只加载公钥（仅验证模式）.
func WithECDSAKeyFiles(privateKeyPath, publicKeyPath string) Option {
	return func(o *options) {
		o.signingMethod = jwt.SigningMethodES256
		if privateKeyPath != "" {
			privKey, err := LoadECDSAPrivateKeyFile(privateKeyPath)
			if err != nil {
				panic("jwt: 加载 ECDSA 私钥文件失败: " + err.Error())
			}
			o.privateKey = privKey
		}
		pubKey, err := LoadECDSAPublicKeyFile(publicKeyPath)
		if err != nil {
			panic("jwt: 加载 ECDSA 公钥文件失败: " + err.Error())
		}
		o.publicKey = pubKey
	}
}

// WithEdDSAKeyFiles 从 PEM 文件加载 Ed25519 密钥对（EdDSA）.
//
// 如果 privateKeyPath 为空，则只加载公钥（仅验证模式）.
func WithEdDSAKeyFiles(privateKeyPath, publicKeyPath string) Option {
	return func(o *options) {
		o.signingMethod = jwt.SigningMethodEdDSA
		if privateKeyPath != "" {
			privKey, err := LoadEdDSAPrivateKeyFile(privateKeyPath)
			if err != nil {
				panic("jwt: 加载 Ed25519 私钥文件失败: " + err.Error())
			}
			o.privateKey = privKey
		}
		pubKey, err := LoadEdDSAPublicKeyFile(publicKeyPath)
		if err != nil {
			panic("jwt: 加载 Ed25519 公钥文件失败: " + err.Error())
		}
		o.publicKey = pubKey
	}
}
