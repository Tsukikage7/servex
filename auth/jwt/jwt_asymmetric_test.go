package jwt_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Tsukikage7/servex/auth/jwt"
	"github.com/Tsukikage7/servex/observability/logger"
)

// newTestLogger 创建测试用 logger.
func newTestLogger(t *testing.T) logger.Logger {
	t.Helper()
	return logger.MustNewLogger(&logger.Config{Level: "debug", Format: "console", Output: "console"})
}

// newTestClaims 创建测试用 Claims.
func newTestClaims() *jwt.StandardClaims {
	now := time.Now()
	return &jwt.StandardClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "test-service",
			ExpiresAt: gojwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(now),
		},
	}
}

// --- RSA (RS256) 测试 ---

func TestRSA_GenerateAndValidate(t *testing.T) {
	// 生成 RSA 密钥对
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 RSA 密钥失败: %v", err)
	}

	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithRSAKeys(privKey, &privKey.PublicKey),
		jwt.WithIssuer("test-service"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	claims := newTestClaims()

	// 生成令牌
	token, err := j.Generate(ctx, claims)
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}
	if token == "" {
		t.Fatal("令牌不应为空")
	}

	// 验证令牌
	parsed, err := j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}

	subject, _ := parsed.GetSubject()
	if subject != "user-123" {
		t.Errorf("期望 subject=user-123，实际=%s", subject)
	}
}

func TestRSA_PublicKeyOnlyValidation(t *testing.T) {
	// 模拟分布式场景：服务A签名，服务B仅用公钥验证
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 RSA 密钥失败: %v", err)
	}

	log := newTestLogger(t)

	// 服务A：用私钥签名
	signer := jwt.NewJWT(
		jwt.WithRSAKeys(privKey, &privKey.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, err := signer.Generate(ctx, newTestClaims())
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	// 服务B：仅用公钥验证（privateKey 为 nil）
	verifier := jwt.NewJWT(
		jwt.WithRSAKeys(nil, &privKey.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	parsed, err := verifier.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("公钥验证失败: %v", err)
	}

	subject, _ := parsed.GetSubject()
	if subject != "user-123" {
		t.Errorf("期望 subject=user-123，实际=%s", subject)
	}
}

func TestRSA_WrongKeyRejectsToken(t *testing.T) {
	// 用一个私钥签名，用另一个公钥验证 -> 应该失败
	privKey1, _ := rsa.GenerateKey(rand.Reader, 2048)
	privKey2, _ := rsa.GenerateKey(rand.Reader, 2048)

	log := newTestLogger(t)

	signer := jwt.NewJWT(
		jwt.WithRSAKeys(privKey1, &privKey1.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, err := signer.Generate(ctx, newTestClaims())
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	// 用不同公钥验证
	verifier := jwt.NewJWT(
		jwt.WithRSAKeys(nil, &privKey2.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	_, err = verifier.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err == nil {
		t.Fatal("使用错误公钥应验证失败")
	}
}

// --- ECDSA (ES256) 测试 ---

func TestECDSA_GenerateAndValidate(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成 ECDSA 密钥失败: %v", err)
	}

	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithECDSAKeys(privKey, &privKey.PublicKey),
		jwt.WithIssuer("test-service"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	claims := newTestClaims()

	token, err := j.Generate(ctx, claims)
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	parsed, err := j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}

	subject, _ := parsed.GetSubject()
	if subject != "user-123" {
		t.Errorf("期望 subject=user-123，实际=%s", subject)
	}
}

func TestECDSA_WrongKeyRejectsToken(t *testing.T) {
	privKey1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	privKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	log := newTestLogger(t)

	signer := jwt.NewJWT(
		jwt.WithECDSAKeys(privKey1, &privKey1.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, _ := signer.Generate(ctx, newTestClaims())

	verifier := jwt.NewJWT(
		jwt.WithECDSAKeys(nil, &privKey2.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	_, err := verifier.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err == nil {
		t.Fatal("使用错误公钥应验证失败")
	}
}

// --- EdDSA (Ed25519) 测试 ---

func TestEdDSA_GenerateAndValidate(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成 Ed25519 密钥失败: %v", err)
	}

	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithEdDSAKeys(privKey, pubKey),
		jwt.WithIssuer("test-service"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	claims := newTestClaims()

	token, err := j.Generate(ctx, claims)
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	parsed, err := j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}

	subject, _ := parsed.GetSubject()
	if subject != "user-123" {
		t.Errorf("期望 subject=user-123，实际=%s", subject)
	}
}

func TestEdDSA_PublicKeyOnlyValidation(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	log := newTestLogger(t)

	signer := jwt.NewJWT(
		jwt.WithEdDSAKeys(privKey, pubKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, _ := signer.Generate(ctx, newTestClaims())

	// 仅公钥验证
	verifier := jwt.NewJWT(
		jwt.WithEdDSAKeys(nil, pubKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	parsed, err := verifier.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("公钥验证失败: %v", err)
	}

	subject, _ := parsed.GetSubject()
	if subject != "user-123" {
		t.Errorf("期望 subject=user-123，实际=%s", subject)
	}
}

// --- 刷新令牌测试 ---

func TestRSA_RefreshWithClaims(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	log := newTestLogger(t)

	j := jwt.NewJWT(
		jwt.WithRSAKeys(privKey, &privKey.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
		jwt.WithRefreshWindow(1*time.Hour),
	)

	ctx := t.Context()

	// 创建一个即将过期的令牌
	now := time.Now()
	claims := &jwt.StandardClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-456",
			Issuer:    "test-service",
			ExpiresAt: gojwt.NewNumericDate(now.Add(1 * time.Minute)),
			IssuedAt:  gojwt.NewNumericDate(now),
		},
	}

	token, err := j.Generate(ctx, claims)
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	// 刷新令牌
	newClaims := &jwt.StandardClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-456",
			Issuer:    "test-service",
			ExpiresAt: gojwt.NewNumericDate(now.Add(2 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(now),
		},
	}

	newToken, err := j.RefreshWithClaims(ctx, token, &jwt.StandardClaims{}, newClaims)
	if err != nil {
		t.Fatalf("刷新令牌失败: %v", err)
	}

	if newToken == "" {
		t.Fatal("新令牌不应为空")
	}
	if newToken == token {
		t.Error("新旧令牌不应相同")
	}
}

// --- HMAC 向后兼容性测试 ---

func TestHMAC_BackwardCompatibility(t *testing.T) {
	log := newTestLogger(t)

	j := jwt.NewJWT(
		jwt.WithSecretKey("my-secret-key-at-least-32-bytes!"),
		jwt.WithIssuer("test-service"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	claims := newTestClaims()

	token, err := j.Generate(ctx, claims)
	if err != nil {
		t.Fatalf("HMAC 生成令牌失败: %v", err)
	}

	parsed, err := j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("HMAC 验证令牌失败: %v", err)
	}

	subject, _ := parsed.GetSubject()
	if subject != "user-123" {
		t.Errorf("期望 subject=user-123，实际=%s", subject)
	}
}

// --- 算法不匹配拒绝测试 ---

func TestAlgorithmMismatchRejected(t *testing.T) {
	// RS256 签名的令牌不应被 HMAC 验证器接受
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	log := newTestLogger(t)

	rsaSigner := jwt.NewJWT(
		jwt.WithRSAKeys(privKey, &privKey.PublicKey),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, _ := rsaSigner.Generate(ctx, newTestClaims())

	hmacVerifier := jwt.NewJWT(
		jwt.WithSecretKey("my-secret-key-at-least-32-bytes!"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	_, err := hmacVerifier.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err == nil {
		t.Fatal("RS256 令牌不应被 HMAC 验证器接受")
	}
}

// --- PEM 密钥加载测试 ---

func TestLoadRSAKeysFromPEM(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	// 编码为 PEM
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("编码公钥失败: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	// 从 PEM 加载
	loadedPriv, err := jwt.LoadRSAPrivateKey(privPEM)
	if err != nil {
		t.Fatalf("加载 RSA 私钥失败: %v", err)
	}

	loadedPub, err := jwt.LoadRSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("加载 RSA 公钥失败: %v", err)
	}

	// 用加载的密钥签名和验证
	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithRSAKeys(loadedPriv, loadedPub),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, err := j.Generate(ctx, newTestClaims())
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	_, err = j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}
}

func TestLoadECDSAKeysFromPEM(t *testing.T) {
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	privDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		t.Fatalf("编码 ECDSA 私钥失败: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privDER,
	})

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("编码公钥失败: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	loadedPriv, err := jwt.LoadECDSAPrivateKey(privPEM)
	if err != nil {
		t.Fatalf("加载 ECDSA 私钥失败: %v", err)
	}

	loadedPub, err := jwt.LoadECDSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("加载 ECDSA 公钥失败: %v", err)
	}

	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithECDSAKeys(loadedPriv, loadedPub),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, err := j.Generate(ctx, newTestClaims())
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	_, err = j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}
}

func TestLoadEdDSAKeysFromPEM(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)

	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("编码 Ed25519 私钥失败: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privDER,
	})

	pubDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("编码公钥失败: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	loadedPriv, err := jwt.LoadEdDSAPrivateKey(privPEM)
	if err != nil {
		t.Fatalf("加载 Ed25519 私钥失败: %v", err)
	}

	loadedPub, err := jwt.LoadEdDSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("加载 Ed25519 公钥失败: %v", err)
	}

	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithEdDSAKeys(loadedPriv, loadedPub),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, err := j.Generate(ctx, newTestClaims())
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	_, err = j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}
}

// --- PEM 文件加载测试 ---

func TestRSAKeyFiles(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	tmpDir := t.TempDir()

	// 写入私钥文件
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	privPath := filepath.Join(tmpDir, "rsa_priv.pem")
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	// 写入公钥文件
	pubDER, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})
	pubPath := filepath.Join(tmpDir, "rsa_pub.pem")
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	log := newTestLogger(t)
	j := jwt.NewJWT(
		jwt.WithRSAKeyFiles(privPath, pubPath),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	ctx := t.Context()
	token, err := j.Generate(ctx, newTestClaims())
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}

	_, err = j.ValidateWithClaims(ctx, token, &jwt.StandardClaims{})
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}
}

// --- PEM 加载错误测试 ---

func TestLoadRSAPrivateKey_InvalidPEM(t *testing.T) {
	_, err := jwt.LoadRSAPrivateKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("无效 PEM 应返回错误")
	}
}

func TestLoadRSAPublicKey_InvalidPEM(t *testing.T) {
	_, err := jwt.LoadRSAPublicKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("无效 PEM 应返回错误")
	}
}

func TestLoadECDSAPrivateKey_InvalidPEM(t *testing.T) {
	_, err := jwt.LoadECDSAPrivateKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("无效 PEM 应返回错误")
	}
}

func TestLoadECDSAPublicKey_InvalidPEM(t *testing.T) {
	_, err := jwt.LoadECDSAPublicKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("无效 PEM 应返回错误")
	}
}

func TestLoadEdDSAPrivateKey_InvalidPEM(t *testing.T) {
	_, err := jwt.LoadEdDSAPrivateKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("无效 PEM 应返回错误")
	}
}

func TestLoadEdDSAPublicKey_InvalidPEM(t *testing.T) {
	_, err := jwt.LoadEdDSAPublicKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("无效 PEM 应返回错误")
	}
}

// --- NewJWT panic 测试 ---

func TestNewJWT_PanicNoPublicKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("非对称模式缺少公钥应 panic")
		}
	}()

	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	log := newTestLogger(t)

	jwt.NewJWT(
		jwt.WithRSAKeys(privKey, nil),
		jwt.WithLogger(log),
	)
}

func TestNewJWT_PanicNoKeyConfig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("无密钥配置应 panic")
		}
	}()

	log := newTestLogger(t)
	jwt.NewJWT(
		jwt.WithLogger(log),
	)
}

// --- 类型不匹配 PEM 测试 ---

func TestLoadRSAPrivateKey_WrongKeyType(t *testing.T) {
	// 用 ECDSA 私钥的 PKCS#8 PEM 传给 RSA 加载器
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(ecKey)
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err := jwt.LoadRSAPrivateKey(pemData)
	if err == nil {
		t.Fatal("ECDSA 密钥传给 RSA 加载器应返回错误")
	}
}

func TestLoadECDSAPrivateKey_WrongKeyType(t *testing.T) {
	// 用 RSA 私钥的 PKCS#8 PEM 传给 ECDSA 加载器
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err := jwt.LoadECDSAPrivateKey(pemData)
	if err == nil {
		t.Fatal("RSA 密钥传给 ECDSA 加载器应返回错误")
	}
}

func TestLoadEdDSAPrivateKey_WrongKeyType(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err := jwt.LoadEdDSAPrivateKey(pemData)
	if err == nil {
		t.Fatal("RSA 密钥传给 EdDSA 加载器应返回错误")
	}
}
