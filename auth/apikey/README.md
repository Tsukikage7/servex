# auth/apikey

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/auth/apikey"
```

## 简介

`auth/apikey` 提供基于 API Key 的认证器实现，实现 `auth.Authenticator` 接口。支持静态键映射和缓存两种验证方式，适用于服务间调用或对外开放的 API 接入场景。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Authenticator` | API Key 认证器，实现 `auth.Authenticator` |
| `Validator` | 验证函数类型 `func(ctx, key) (*auth.Principal, error)` |
| `New(validator)` | 创建认证器 |
| `StaticValidator(keys)` | 静态键映射验证器，适合键数量固定的场景 |
| `CacheValidator(cache, ttl)` | 从缓存读取 Principal 的验证器，适合动态发放场景 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/Tsukikage7/servex/v2/auth"
    "github.com/Tsukikage7/servex/v2/auth/apikey"
)

func main() {
    // 静态 API Key 映射
    keys := map[string]*auth.Principal{
        "secret-key-1": {
            ID:   "service-a",
            Type: auth.PrincipalTypeService,
        },
        "secret-key-2": {
            ID:   "service-b",
            Type: auth.PrincipalTypeService,
        },
    }

    authenticator := apikey.New(apikey.StaticValidator(keys))

    // 验证请求
    creds := auth.Credentials{
        Type:  auth.CredentialTypeAPIKey,
        Token: "secret-key-1",
    }

    principal, err := authenticator.Authenticate(context.Background(), creds)
    if err != nil {
        fmt.Println("认证失败:", err)
        return
    }
    fmt.Println("认证成功，Principal ID:", principal.ID)

    // 自定义验证器
    customValidator := apikey.Validator(func(ctx context.Context, key string) (*auth.Principal, error) {
        // 从数据库或外部服务查询
        if key == "dynamic-key" {
            return &auth.Principal{ID: "user-123", Type: auth.PrincipalTypeUser}, nil
        }
        return nil, auth.ErrInvalidCredentials
    })

    _ = apikey.New(customValidator)
    _ = http.ListenAndServe(":8080", nil)
}
```
