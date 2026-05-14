# Discovery 服务发现包

提供微服务架构中的服务注册与发现抽象、配置和工厂。Consul、etcd、Nacos provider 放在独立子包中，通过 blank import 按需注册，避免只使用抽象时拉入所有注册中心 SDK。

## 功能特性

- 服务注册与注销
- 服务发现
- Config 驱动工厂
- Provider 按需注册
- 多协议支持（HTTP/gRPC）
- 健康检查配置
- 中文错误信息

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/discovery
```

## API 参考

### 类型常量

```go
const (
    TypeConsul = "consul"
    TypeEtcd   = "etcd"
    TypeNacos  = "nacos"
)

const (
    ProtocolHTTP = "http"  // HTTP 协议
    ProtocolGRPC = "grpc"  // gRPC 协议
)
```

### 按需注册 provider

```go
import (
    "github.com/Tsukikage7/servex/v2/discovery"
    _ "github.com/Tsukikage7/servex/v2/discovery/consul"
)

d, err := discovery.NewDiscovery(&discovery.Config{
    Type: discovery.TypeConsul,
    Addr: "127.0.0.1:8500",
}, log)
```

### 默认值

| 配置项         | 默认值 |
| -------------- | ------ |
| 健康检查间隔   | 10s    |
| 健康检查超时   | 3s     |
| 失败后注销时间 | 30s    |
| 服务版本       | 1.0.0  |

### 错误类型

| 错误                     | 说明                 |
| ------------------------ | -------------------- |
| `ErrNilConfig`           | 配置为空             |
| `ErrNilLogger`           | 日志记录器为空       |
| `ErrEmptyName`           | 服务名称为空         |
| `ErrEmptyAddress`        | 服务地址为空         |
| `ErrEmptyServiceID`      | 服务ID为空           |
| `ErrUnsupportedType`     | 不支持的服务发现类型 |
| `ErrUnsupportedProtocol` | 不支持的协议类型     |
| `ErrInvalidAddress`      | 无效的地址格式       |
| `ErrInvalidPort`         | 无效的端口号         |
| `ErrNotFound`            | 未发现任何服务实例   |

## 文件结构

```
discovery/
├── discovery.go       # 接口定义
├── config.go          # 配置结构体
├── factory.go         # 工厂和 provider 注册表
├── consul/            # Consul provider
├── etcd/              # etcd provider
├── nacos/             # Nacos provider
└── README.md
```

## 测试

```bash
# 运行测试
go test -v ./discovery ./discovery/consul ./discovery/etcd ./discovery/nacos

# 运行测试并查看覆盖率
go test -v ./discovery ./discovery/consul ./discovery/etcd ./discovery/nacos -cover

# 生成覆盖率报告
go test ./discovery ./discovery/consul ./discovery/etcd ./discovery/nacos -coverprofile=coverage.out
go tool cover -html=coverage.out
```
