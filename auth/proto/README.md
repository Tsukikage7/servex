# auth/proto

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/auth/proto"
// Go package 名：authpb
```

## 简介

`auth/proto` 包含认证相关的 Protobuf 定义（`auth.proto`），通过扩展 `google.protobuf.MethodOptions` 和 `google.protobuf.ServiceOptions` 为 gRPC 服务提供方法级和服务级的声明式认证配置，配合 `auth` 包中间件在服务端解析并执行权限校验。

## Protobuf 消息

| 消息 / 扩展 | 说明 |
|---|---|
| `MethodAuthOptions` | 方法级认证选项：`public`（公开）、`permissions`（权限列表）、`all_permissions`（AND 逻辑） |
| `ServiceAuthOptions` | 服务级认证选项：`public`（整个服务公开）、`default_permissions`（默认权限） |
| `method`（扩展 50001） | 挂载到 `google.protobuf.MethodOptions` |
| `service`（扩展 50002） | 挂载到 `google.protobuf.ServiceOptions` |

## 示例

在 `.proto` 文件中使用：

```protobuf
syntax = "proto3";

import "auth/proto/auth.proto";

// 整个服务默认需要认证
service UserService {
    option (microservice.kit.auth.service) = {
        default_permissions: ["user:read"]
    };

    // 登录接口不需要认证
    rpc Login(LoginRequest) returns (LoginResponse) {
        option (microservice.kit.auth.method) = {
            public: true
        };
    }

    // 需要读取权限（继承服务默认）
    rpc GetProfile(GetProfileRequest) returns (GetProfileResponse) {}

    // 需要同时拥有两个权限（AND 逻辑）
    rpc DeleteAccount(DeleteAccountRequest) returns (DeleteAccountResponse) {
        option (microservice.kit.auth.method) = {
            permissions: ["user:write", "admin"]
            all_permissions: true
        };
    }
}

// 完全公开的服务
service PublicService {
    option (microservice.kit.auth.service) = {
        public: true
    };

    rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse) {}
}
```

在 Gateway 中启用声明式策略：

```go
srv := gateway.New(
    gateway.WithSecurity(gateway.SecurityConfig{
        Authenticator: authenticator,
        AuthOptions:   []auth.Option{auth.WithAuthorizer(authorizer)},
    }),
)
```

`public: true` 会跳过认证；`permissions` 会在认证后交给显式配置的
`auth.Authorizer` 执行授权；`all_permissions: true` 表示必须满足全部权限。
没有 proto option 的方法不会被公开，会按普通认证接口处理。如果声明了 `permissions`
但没有配置授权器，请求会被拒绝。

在 Go 代码中手动读取选项：

```go
import authpb "github.com/Tsukikage7/servex/v2/auth/proto"
import "google.golang.org/protobuf/proto"

opts := method.Desc.Options()
authOpts := proto.GetExtension(opts, authpb.E_Method).(*authpb.MethodAuthOptions)
if authOpts.GetPublic() {
    // 跳过认证
}
```
