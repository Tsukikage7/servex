package tlsx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateSelfSignedCert 生成自签名证书用于测试.
func generateSelfSignedCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	// 生成 ECDSA 密钥
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// 创建证书模板
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// 自签名
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	// 写入证书文件
	certFile = filepath.Join(dir, "cert.pem")
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)
	certOut.Close()

	// 写入密钥文件
	keyFile = filepath.Join(dir, "key.pem")
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, err)
	keyOut.Close()

	return certFile, keyFile
}

func TestNewTLSConfig_NilConfig(t *testing.T) {
	_, err := NewTLSConfig(nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestNewTLSConfig_MissingCert(t *testing.T) {
	_, err := NewTLSConfig(&Config{KeyFile: "key.pem"})
	assert.ErrorIs(t, err, ErrMissingCert)
}

func TestNewTLSConfig_MissingKey(t *testing.T) {
	_, err := NewTLSConfig(&Config{CertFile: "cert.pem"})
	assert.ErrorIs(t, err, ErrMissingKey)
}

func TestNewServerTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSignedCert(t, dir)

	cfg := &Config{
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	tlsCfg, err := NewServerTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsCfg)
	assert.Len(t, tlsCfg.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsCfg.MinVersion)
}

func TestNewServerTLSConfig_WithCA(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSignedCert(t, dir)

	cfg := &Config{
		CertFile:   certFile,
		KeyFile:    keyFile,
		CAFile:     certFile, // 使用同一证书作为 CA测试用
		ClientAuth: "require_and_verify",
	}

	tlsCfg, err := NewServerTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsCfg.ClientCAs)
	assert.Equal(t, tls.RequireAndVerifyClientCert, tlsCfg.ClientAuth)
}

func TestNewClientTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSignedCert(t, dir)

	cfg := &Config{
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   certFile,
	}

	tlsCfg, err := NewClientTLSConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, tlsCfg)
	assert.Len(t, tlsCfg.Certificates, 1)
	assert.NotNil(t, tlsCfg.RootCAs)
}

func TestNewClientTLSConfig_NoCert(t *testing.T) {
	dir := t.TempDir()
	certFile, _ := generateSelfSignedCert(t, dir)

	cfg := &Config{
		CAFile:             certFile,
		InsecureSkipVerify: true,
	}

	tlsCfg, err := NewClientTLSConfig(cfg)
	require.NoError(t, err)
	assert.Empty(t, tlsCfg.Certificates)
	assert.True(t, tlsCfg.InsecureSkipVerify)
	assert.NotNil(t, tlsCfg.RootCAs)
}

func TestNewClientTLSConfig_NilConfig(t *testing.T) {
	_, err := NewClientTLSConfig(nil)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestMinVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint16
	}{
		{name: "默认TLS1.2", input: "", want: tls.VersionTLS12},
		{name: "TLS1.0", input: "1.0", want: tls.VersionTLS10},
		{name: "TLS1.1", input: "1.1", want: tls.VersionTLS11},
		{name: "TLS1.2", input: "1.2", want: tls.VersionTLS12},
		{name: "TLS1.3", input: "1.3", want: tls.VersionTLS13},
		{name: "TLS1.3字符串", input: "TLS1.3", want: tls.VersionTLS13},
		{name: "未知版本默认1.2", input: "unknown", want: tls.VersionTLS12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMinVersion(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseClientAuth(t *testing.T) {
	tests := []struct {
		input string
		want  tls.ClientAuthType
	}{
		{"", tls.NoClientCert},
		{"request", tls.RequestClientCert},
		{"require", tls.RequireAnyClientCert},
		{"verify", tls.VerifyClientCertIfGiven},
		{"require_and_verify", tls.RequireAndVerifyClientCert},
		{"RequireAndVerifyClientCert", tls.RequireAndVerifyClientCert},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseClientAuth(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
