// Package tlsx 提供 TLS 配置工具.
//
// 简化服务端/客户端 TLS 配置的创建，支持 mTLS（双向 TLS）.
// 包名使用 tlsx 以避免与标准库 crypto/tls 冲突.
package tlsx

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
)

var (
	// ErrNilConfig 配置为 nil.
	ErrNilConfig = errors.New("tls: config is nil")
	// ErrMissingCert 缺少证书文件.
	ErrMissingCert = errors.New("tls: cert_file is required")
	// ErrMissingKey 缺少密钥文件.
	ErrMissingKey = errors.New("tls: key_file is required")
)

// Config TLS 配置.
type Config struct {
	CertFile string `json:"cert_file" yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile  string `json:"key_file" yaml:"key_file" mapstructure:"key_file"`
	CAFile   string `json:"ca_file" yaml:"ca_file" mapstructure:"ca_file"` // 用于 mTLS
	// MinVersion 最低 TLS 版本，默认 TLS 1.2
	MinVersion string `json:"min_version" yaml:"min_version" mapstructure:"min_version"`
	// ClientAuth 客户端认证模式，默认 NoClientCert
	ClientAuth string `json:"client_auth" yaml:"client_auth" mapstructure:"client_auth"`
	// InsecureSkipVerify 跳过证书验证（仅测试用），默认 false
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify" mapstructure:"insecure_skip_verify"`
}

// NewTLSConfig 从 Config 创建 *tls.Config.
//
// 加载证书密钥对，可选加载 CA 证书用于 mTLS.
func NewTLSConfig(cfg *Config) (*tls.Config, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if cfg.CertFile == "" {
		return nil, ErrMissingCert
	}
	if cfg.KeyFile == "" {
		return nil, ErrMissingKey
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("tls: failed to load key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   parseMinVersion(cfg.MinVersion),
	}

	// 加载 CA 证书（用于验证对端证书）
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tls: failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("tls: failed to parse CA certificate")
		}
		tlsCfg.RootCAs = pool
		tlsCfg.ClientCAs = pool
	}

	tlsCfg.ClientAuth = parseClientAuth(cfg.ClientAuth)

	if cfg.InsecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true
	}

	return tlsCfg, nil
}

// NewServerTLSConfig 创建服务端 TLS 配置.
//
// 与 NewTLSConfig 行为一致，但语义上明确用于服务端.
func NewServerTLSConfig(cfg *Config) (*tls.Config, error) {
	return NewTLSConfig(cfg)
}

// NewClientTLSConfig 创建客户端 TLS 配置（用于 mTLS 客户端）.
//
// 如果未提供 cert/key，仅配置 CA 和最低版本（普通 TLS 客户端）.
// 如果提供了 cert/key，同时加载客户端证书（mTLS 客户端）.
func NewClientTLSConfig(cfg *Config) (*tls.Config, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	tlsCfg := &tls.Config{
		MinVersion:         parseMinVersion(cfg.MinVersion),
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	// 加载客户端证书（mTLS）
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("tls: failed to load client key pair: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// 加载 CA 证书
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tls: failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("tls: failed to parse CA certificate")
		}
		tlsCfg.RootCAs = pool
	}

	return tlsCfg, nil
}

// parseMinVersion 解析最低 TLS 版本字符串.
func parseMinVersion(v string) uint16 {
	switch v {
	case "1.0", "TLS1.0":
		log.Println("WARNING: TLS 1.0 is deprecated by RFC 8996 and should not be used. Please upgrade to TLS 1.2 or higher.")
		return tls.VersionTLS10
	case "1.1", "TLS1.1":
		log.Println("WARNING: TLS 1.1 is deprecated by RFC 8996 and should not be used. Please upgrade to TLS 1.2 or higher.")
		return tls.VersionTLS11
	case "1.3", "TLS1.3":
		return tls.VersionTLS13
	default:
		// 默认 TLS 1.2
		return tls.VersionTLS12
	}
}

// parseClientAuth 解析客户端认证模式字符串.
func parseClientAuth(s string) tls.ClientAuthType {
	switch s {
	case "request", "RequestClientCert":
		return tls.RequestClientCert
	case "require", "RequireAnyClientCert":
		return tls.RequireAnyClientCert
	case "verify", "VerifyClientCertIfGiven":
		return tls.VerifyClientCertIfGiven
	case "require_and_verify", "RequireAndVerifyClientCert":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}
