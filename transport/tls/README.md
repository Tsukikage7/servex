# transport/tls (tlsx)

TLS 配置工具包，简化服务端/客户端 `*tls.Config` 的创建，支持 mTLS（双向 TLS）和版本控制。

包名使用 `tlsx` 以避免与标准库 `crypto/tls` 冲突。

## 快速开始

### 服务端 TLS（HTTPS / gRPCS）

```go
import (
    "github.com/Tsukikage7/servex/v2/transport/tls"
    "github.com/Tsukikage7/servex/v2/transport/httpserver"
    "github.com/Tsukikage7/servex/v2/transport/grpcserver"
)

tlsCfg, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/server.crt",
    KeyFile:  "/etc/tls/server.key",
})
if err != nil {
    log.Fatal(err)
}

// HTTP 服务器启用 TLS
srv := httpserver.New(mux,
    httpserver.WithAddr(":443"),
    httpserver.WithTLS(tlsCfg),
)

// gRPC 服务器启用 TLS
grpcSrv := grpcserver.New(
    grpcserver.WithAddr(":9443"),
    grpcserver.WithTLS(tlsCfg),
)
```

### 客户端 TLS（HTTPS 客户端）

```go
tlsCfg, err := tlsx.NewClientTLSConfig(&tlsx.Config{
    CAFile: "/etc/tls/ca.crt",  // 验证服务端证书
})
if err != nil {
    log.Fatal(err)
}

httpClient := &http.Client{
    Transport: &http.Transport{TLSClientConfig: tlsCfg},
}
```

### mTLS（双向 TLS）

```go
// 服务端：要求并验证客户端证书
serverTLS, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile:   "/etc/tls/server.crt",
    KeyFile:    "/etc/tls/server.key",
    CAFile:     "/etc/tls/ca.crt",      // 客户端证书签发 CA
    ClientAuth: "require_and_verify",    // 强制双向验证
})

// 客户端：提供客户端证书
clientTLS, err := tlsx.NewClientTLSConfig(&tlsx.Config{
    CertFile: "/etc/tls/client.crt",
    KeyFile:  "/etc/tls/client.key",
    CAFile:   "/etc/tls/ca.crt",        // 验证服务端证书
})
```

### 指定最低 TLS 版本

```go
tlsCfg, err := tlsx.NewServerTLSConfig(&tlsx.Config{
    CertFile:   "server.crt",
    KeyFile:    "server.key",
    MinVersion: "1.3",  // 仅允许 TLS 1.3
})
```

## Config 字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `CertFile` | `string` | 必填（服务端） | 证书文件路径（PEM） |
| `KeyFile` | `string` | 必填（服务端） | 私钥文件路径（PEM） |
| `CAFile` | `string` | `""` | CA 证书路径，用于 mTLS 或客户端验证服务端 |
| `MinVersion` | `string` | `"1.2"` | 最低 TLS 版本：`"1.0"`, `"1.1"`, `"1.2"`, `"1.3"` |
| `ClientAuth` | `string` | `"no"` | 客户端认证模式（见下表） |
| `InsecureSkipVerify` | `bool` | `false` | 跳过证书验证（仅用于测试） |

### ClientAuth 可选值

| 值 | 对应常量 | 说明 |
|----|----------|------|
| `""` / `"no"` | `NoClientCert` | 不验证客户端证书（默认） |
| `"request"` | `RequestClientCert` | 请求客户端证书，但不强制 |
| `"require"` | `RequireAnyClientCert` | 强制客户端提供证书，但不验证 |
| `"verify"` | `VerifyClientCertIfGiven` | 若提供证书则验证 |
| `"require_and_verify"` | `RequireAndVerifyClientCert` | 强制提供并验证（mTLS） |

## Config 支持结构体标签

`Config` 字段支持 `json`、`yaml`、`mapstructure` 标签，可直接与配置管理（`config` 包）集成：

```yaml
tls:
  cert_file: /etc/tls/server.crt
  key_file: /etc/tls/server.key
  ca_file: /etc/tls/ca.crt
  min_version: "1.2"
  client_auth: require_and_verify
```

## 预定义错误

```go
tlsx.ErrNilConfig    // cfg 为 nil
tlsx.ErrMissingCert  // cert_file 未提供
tlsx.ErrMissingKey   // key_file 未提供
```

## API

```go
// 创建通用 TLS 配置（服务端，需要 CertFile + KeyFile）
func NewTLSConfig(cfg *Config) (*tls.Config, error)

// 语义等同 NewTLSConfig，明确用于服务端
func NewServerTLSConfig(cfg *Config) (*tls.Config, error)

// 创建客户端 TLS 配置（CertFile/KeyFile 可选，用于 mTLS）
func NewClientTLSConfig(cfg *Config) (*tls.Config, error)
```
