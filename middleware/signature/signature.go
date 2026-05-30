// Package signature 提供 HMAC 请求签名验证中间件.
// 特性：
//   - HMAC-SHA256/SHA512 签名
//   - 时间戳防重放攻击
//   - HTTP 中间件服务端验签
//   - 请求签名辅助函数客户端签名
//   - 常量时间比较防时序攻击
//
// 签名算法：
//
//	HMAC-SHA256(secret, timestamp + "." + body)
//
// 示例：
//
//	// 服务端
//	cfg := signature.DefaultConfig("my-secret")
//	handler = signature.HTTPMiddleware(cfg)(handler)
//	// 客户端
//	req, _ := http.NewRequest("POST", url, body)
//	_ = signature.SignRequest(req, "my-secret")
package signature

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

var (
	// ErrMissingSignature 缺少签名头.
	ErrMissingSignature = errors.New("signature: missing signature header")
	// ErrMissingTimestamp 缺少时间戳头.
	ErrMissingTimestamp = errors.New("signature: missing timestamp header")
	// ErrExpiredTimestamp 时间戳已过期.
	ErrExpiredTimestamp = errors.New("signature: timestamp expired")
	// ErrInvalidSignature 签名无效.
	ErrInvalidSignature = errors.New("signature: invalid signature")
)

// Config 签名配置.
type Config struct {
	Secret          string        // HMAC 密钥
	HeaderName      string        // 签名头名，默认 "X-Signature"
	TimestampHeader string        // 时间戳头名，默认 "X-Timestamp"
	MaxAge          time.Duration // 签名最大有效期，默认 5 分钟防重放
	Algorithm       string        // "sha256" (默认) 或 "sha512"
	MaxBodySize     int64         // 请求体最大字节数，默认 1MB0 表示不限制
}

// DefaultConfig 创建默认签名配置.
func DefaultConfig(secret string) *Config {
	return &Config{
		Secret:          secret,
		HeaderName:      "X-Signature",
		TimestampHeader: "X-Timestamp",
		MaxAge:          5 * time.Minute,
		Algorithm:       "sha256",
		MaxBodySize:     1 << 20, // 1MB
	}
}

// applyDefaults 填充默认值.
func (c *Config) applyDefaults() {
	if c.HeaderName == "" {
		c.HeaderName = "X-Signature"
	}
	if c.TimestampHeader == "" {
		c.TimestampHeader = "X-Timestamp"
	}
	if c.MaxAge == 0 {
		c.MaxAge = 5 * time.Minute
	}
	if c.Algorithm == "" {
		c.Algorithm = "sha256"
	}
}

// newHMAC 根据算法创建 HMAC hash.
func newHMAC(algorithm string, secret []byte) hash.Hash {
	switch algorithm {
	case "sha512":
		return hmac.New(sha512.New, secret)
	default:
		return hmac.New(sha256.New, secret)
	}
}

// Sign 对请求体签名.
// 签名算法: HMAC-SHA256(secret, method + "." + path + "." + timestamp + "." + body)
func Sign(body []byte, timestamp string, secret string) string {
	return signWithAlgorithm(body, "", "", timestamp, secret, "sha256")
}

// SignWithMethodPath 对请求体签名，包含 HTTP 方法和路径.
// 签名算法: HMAC-SHA256(secret, method + "." + path + "." + timestamp + "." + body)
func SignWithMethodPath(body []byte, method, path, timestamp, secret string) string {
	return signWithAlgorithm(body, method, path, timestamp, secret, "sha256")
}

// signWithAlgorithm 使用指定算法签名.
// 签名内容包含 method、path、timestamp 和 body，防止跨方法/路径重放.
func signWithAlgorithm(body []byte, method, path, timestamp string, secret string, algorithm string) string {
	h := newHMAC(algorithm, []byte(secret))
	if method != "" {
		h.Write([]byte(method))
		h.Write([]byte("."))
	}
	if path != "" {
		h.Write([]byte(path))
		h.Write([]byte("."))
	}
	h.Write([]byte(timestamp))
	h.Write([]byte("."))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// Verify 验证签名.
// 使用常量时间比较防止时序攻击.
func Verify(body []byte, timestamp, sig, secret string) bool {
	expected := Sign(body, timestamp, secret)
	return hmac.Equal([]byte(expected), []byte(sig))
}

// VerifyWithMethodPath 验证包含 HTTP 方法和路径的签名.
func VerifyWithMethodPath(body []byte, method, path, timestamp, sig, secret string) bool {
	expected := SignWithMethodPath(body, method, path, timestamp, secret)
	return hmac.Equal([]byte(expected), []byte(sig))
}

// HTTPMiddleware 返回签名验证 HTTP 中间件.
// 流程:
//  1. 读取 body + timestamp header + signature header
//  2. 检查 timestamp 是否在 MaxAge 内
//  3. 验证 HMAC 签名
//  4. 恢复 body 供后续 handler 读取
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler {
	cfg.applyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 读取签名头
			sig := r.Header.Get(cfg.HeaderName)
			if sig == "" {
				http.Error(w, ErrMissingSignature.Error(), http.StatusUnauthorized)
				return
			}

			// 读取时间戳头
			tsStr := r.Header.Get(cfg.TimestampHeader)
			if tsStr == "" {
				http.Error(w, ErrMissingTimestamp.Error(), http.StatusUnauthorized)
				return
			}

			// 检查时间戳有效性
			tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				http.Error(w, ErrExpiredTimestamp.Error(), http.StatusUnauthorized)
				return
			}
			ts := time.Unix(tsUnix, 0)
			diff := time.Since(ts)
			if diff < 0 {
				diff = -diff
			}
			if diff > cfg.MaxAge {
				http.Error(w, ErrExpiredTimestamp.Error(), http.StatusUnauthorized)
				return
			}

			// 读取 body限制大小
			var bodyReader io.Reader = r.Body
			if cfg.MaxBodySize > 0 {
				bodyReader = io.LimitReader(r.Body, cfg.MaxBodySize+1)
			}
			body, err := io.ReadAll(bodyReader)
			if err != nil {
				http.Error(w, "签名验证失败", http.StatusBadRequest)
				return
			}
			if cfg.MaxBodySize > 0 && int64(len(body)) > cfg.MaxBodySize {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			// 恢复 body
			r.Body = io.NopCloser(bytes.NewReader(body))

			// 验证签名包含 HTTP 方法和路径
			expected := signWithAlgorithm(body, r.Method, r.URL.Path, tsStr, cfg.Secret, cfg.Algorithm)
			if !hmac.Equal([]byte(expected), []byte(sig)) {
				http.Error(w, ErrInvalidSignature.Error(), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SignRequest 为 HTTP 请求签名客户端用.
// 读取请求 body，生成时间戳，计算签名，设置 headers.
func SignRequest(req *http.Request, secret string) error {
	return SignRequestWithConfig(req, DefaultConfig(secret))
}

// SignRequestWithConfig 使用指定配置为 HTTP 请求签名.
func SignRequestWithConfig(req *http.Request, cfg *Config) error {
	cfg.applyDefaults()

	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("signature: failed to read request body: %w", err)
		}
		// 恢复 body
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	// 生成时间戳
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// 计算签名包含 HTTP 方法和路径
	sig := signWithAlgorithm(body, req.Method, req.URL.Path, timestamp, cfg.Secret, cfg.Algorithm)

	// 设置 headers
	req.Header.Set(cfg.TimestampHeader, timestamp)
	req.Header.Set(cfg.HeaderName, sig)

	// 如果 ContentLength 需要更新
	if req.ContentLength == 0 && len(body) > 0 {
		req.ContentLength = int64(len(body))
	} else if req.ContentLength < 0 && body != nil {
		// ContentLength == -1 表示未知
		if len(body) <= math.MaxInt64 {
			req.ContentLength = int64(len(body))
		}
	}

	return nil
}
