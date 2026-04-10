package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadRSAPrivateKey 从 PEM 编码的字节中加载 RSA 私钥.
//
// 支持 PKCS#1 和 PKCS#8 格式.
func LoadRSAPrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("jwt: 无法解码 PEM 数据")
	}

	// 优先尝试 PKCS#8
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("jwt: PKCS#8 密钥不是 RSA 类型")
		}
		return rsaKey, nil
	}

	// 回退到 PKCS#1
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// LoadRSAPublicKey 从 PEM 编码的字节中加载 RSA 公钥.
//
// 支持 PKIX 和 PKCS#1 格式.
func LoadRSAPublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("jwt: 无法解码 PEM 数据")
	}

	// 优先尝试 PKIX
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("jwt: PKIX 公钥不是 RSA 类型")
		}
		return rsaKey, nil
	}

	// 回退到 PKCS#1
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

// LoadECDSAPrivateKey 从 PEM 编码的字节中加载 ECDSA 私钥.
//
// 支持 SEC 1 (EC) 和 PKCS#8 格式.
func LoadECDSAPrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("jwt: 无法解码 PEM 数据")
	}

	// 优先尝试 PKCS#8
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		ecKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("jwt: PKCS#8 密钥不是 ECDSA 类型")
		}
		return ecKey, nil
	}

	// 回退到 SEC 1
	return x509.ParseECPrivateKey(block.Bytes)
}

// LoadECDSAPublicKey 从 PEM 编码的字节中加载 ECDSA 公钥.
func LoadECDSAPublicKey(pemData []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("jwt: 无法解码 PEM 数据")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: 解析 ECDSA 公钥失败: %w", err)
	}

	ecKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("jwt: 公钥不是 ECDSA 类型")
	}
	return ecKey, nil
}

// LoadEdDSAPrivateKey 从 PEM 编码的字节中加载 Ed25519 私钥.
//
// 仅支持 PKCS#8 格式.
func LoadEdDSAPrivateKey(pemData []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("jwt: 无法解码 PEM 数据")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: 解析 Ed25519 私钥失败: %w", err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("jwt: PKCS#8 密钥不是 Ed25519 类型")
	}
	return edKey, nil
}

// LoadEdDSAPublicKey 从 PEM 编码的字节中加载 Ed25519 公钥.
func LoadEdDSAPublicKey(pemData []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("jwt: 无法解码 PEM 数据")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: 解析 Ed25519 公钥失败: %w", err)
	}

	edKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("jwt: 公钥不是 Ed25519 类型")
	}
	return edKey, nil
}

// LoadRSAPrivateKeyFile 从 PEM 文件加载 RSA 私钥.
func LoadRSAPrivateKeyFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt: 读取 RSA 私钥文件失败: %w", err)
	}
	return LoadRSAPrivateKey(data)
}

// LoadRSAPublicKeyFile 从 PEM 文件加载 RSA 公钥.
func LoadRSAPublicKeyFile(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt: 读取 RSA 公钥文件失败: %w", err)
	}
	return LoadRSAPublicKey(data)
}

// LoadECDSAPrivateKeyFile 从 PEM 文件加载 ECDSA 私钥.
func LoadECDSAPrivateKeyFile(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt: 读取 ECDSA 私钥文件失败: %w", err)
	}
	return LoadECDSAPrivateKey(data)
}

// LoadECDSAPublicKeyFile 从 PEM 文件加载 ECDSA 公钥.
func LoadECDSAPublicKeyFile(path string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt: 读取 ECDSA 公钥文件失败: %w", err)
	}
	return LoadECDSAPublicKey(data)
}

// LoadEdDSAPrivateKeyFile 从 PEM 文件加载 Ed25519 私钥.
func LoadEdDSAPrivateKeyFile(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt: 读取 Ed25519 私钥文件失败: %w", err)
	}
	return LoadEdDSAPrivateKey(data)
}

// LoadEdDSAPublicKeyFile 从 PEM 文件加载 Ed25519 公钥.
func LoadEdDSAPublicKeyFile(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt: 读取 Ed25519 公钥文件失败: %w", err)
	}
	return LoadEdDSAPublicKey(data)
}
