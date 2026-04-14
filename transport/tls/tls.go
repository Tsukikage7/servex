// Package tlsx 提供 TLS 配置工具.
//
// 简化服务端/客户端 TLS 配置的创建，支持 mTLS（双向 TLS）.
// 包名使用 tlsx 以避免与标准库 crypto/tls 冲突.
package tlsx

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"

	servexerrs "github.com/Tsukikage7/servex/v2/errors"
)

var (
	// ErrNilConfig 配置为 nil.
	ErrNilConfig = servexerrs.New(60701, "transport.tls.nil_config", "TLS 配置为空")
	// ErrMissingCert 缺少证书文件.
	ErrMissingCert = servexerrs.New(60702, "transport.tls.missing_cert", "缺少证书文件")
	// ErrMissingKey 缺少密钥文件.
	ErrMissingKey = servexerrs.New(60703, "transport.tls.missing_key", "缺少密钥文件")
	// ErrLoadKeyPair 加载证书密钥对失败.
	ErrLoadKeyPair = servexerrs.New(60704, "transport.tls.load_key_pair_failed", "加载证书密钥对失败")
	// ErrReadCAFile 读取 CA 证书文件失败.
	ErrReadCAFile = servexerrs.New(60705, "transport.tls.read_ca_file_failed", "读取 CA 证书文件失败")
	// ErrParseCA 解析 CA 证书失败.
	ErrParseCA = servexerrs.New(60706, "transport.tls.parse_ca_failed", "解析 CA 证书失败")
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
		return nil, ErrLoadKeyPair.WithCause(err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   parseMinVersion(cfg.MinVersion),
	}

	// 加载 CA 证书（用于验证对端证书）
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, ErrReadCAFile.WithCause(err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, ErrParseCA
		}
		tlsCfg.RootCAs = pool
		tlsCfg.ClientCAs = pool
	}

	tlsCfg.ClientAuth = parseClientAuth(cfg.ClientAuth)

	if cfg.InsecureSkipVerify {
		slog.Warn("tls: InsecureSkipVerify 已启用，将跳过证书验证，仅应在测试环境中使用")
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

	if cfg.InsecureSkipVerify {
		slog.Warn("tls: InsecureSkipVerify 已启用（客户端），将跳过服务端证书验证，仅应在测试环境中使用")
	}

	// 加载客户端证书（mTLS）
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, ErrLoadKeyPair.WithCause(err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// 加载 CA 证书
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, ErrReadCAFile.WithCause(err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, ErrParseCA
		}
		tlsCfg.RootCAs = pool
	}

	return tlsCfg, nil
}

// parseMinVersion 解析最低 TLS 版本字符串.
func parseMinVersion(v string) uint16 {
	switch v {
	case "1.0", "TLS1.0":
		slog.Warn("TLS 1.0 已被 RFC 8996 弃用，请升级到 TLS 1.2 或更高版本")
		return tls.VersionTLS10
	case "1.1", "TLS1.1":
		slog.Warn("TLS 1.1 已被 RFC 8996 弃用，请升级到 TLS 1.2 或更高版本")
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
