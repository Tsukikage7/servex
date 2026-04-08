# servex 配置与服务发现

## config — 多源配置管理

```go
// 从 YAML 文件加载，带热更新回调
mgr, err := config.NewManager[AppConfig](
    config.WithSource[AppConfig](fileSrc.New("/etc/myapp/config.yaml")),
    config.WithObserver[AppConfig](func(old, new *AppConfig) {
        fmt.Println("配置已更新:", new.Server.Addr)
    }),
)
if err != nil { ... }

// 加载配置（必须在 Watch 之前调用）
if err := mgr.Load(); err != nil { ... }

// 读取当前配置（方法名是 Get，不是 Config）
cfg := mgr.Get()

// 启动热更新（非阻塞）
if err := mgr.Watch(); err != nil { ... }
defer mgr.Close()
```

完整示例：`docs/superpowers/examples/config/main.go`

**注意：** 读取配置的方法是 `Get()`，不是 `Config()`；`Watch()` 是非阻塞的。

## config/source/file — 文件配置源（热更新）

```go
import fileSrc "github.com/Tsukikage7/servex/config/source/file"

// 支持 YAML、JSON、TOML；文件变更自动触发 Observer
src := fileSrc.New("/etc/myapp/config.yaml")
```

## config/source/etcd — etcd 配置源

```go
import etcdSrc "github.com/Tsukikage7/servex/config/source/etcd"

src := etcdSrc.New(etcdSrc.Config{
    Endpoints: []string{"localhost:2379"},
    Key:       "/myapp/config",
})
```

## config/source/env — 环境变量配置源

```go
import envSrc "github.com/Tsukikage7/servex/config/source/env"

// 从环境变量读取，支持监听 .env 文件变更
src := envSrc.New()
```

## config/source/consul — Consul KV 配置源

```go
import (
    consulSrc "github.com/Tsukikage7/servex/config/source/consul"
    "github.com/hashicorp/consul/api"
)

// 创建 Consul 客户端
consulClient, err := api.NewClient(api.DefaultConfig())
if err != nil { ... }

// 创建 Consul KV 配置源
src := consulSrc.New(consulClient, "/myapp/config",
    consulSrc.WithFormat("yaml"),        // 配置格式：json（默认）/ yaml / toml
    consulSrc.WithDatacenter("dc1"),     // 指定数据中心
)

// 与 config.Manager 配合使用
mgr, err := config.NewManager[AppConfig](
    config.WithSource[AppConfig](src),
    config.WithObserver[AppConfig](func(old, new *AppConfig) {
        fmt.Println("Consul 配置已变更")
    }),
)
// Watch() 基于 Consul blocking query（长轮询）实现变更监听
```

**关键类型：**
- `consulSrc.New(client, key, opts...)` — 创建 Consul 配置源
- `WithFormat(format)` — 指定格式（json/yaml/toml），默认 json
- `WithDatacenter(dc)` — 指定 Consul 数据中心
- 支持 `Load()` 和 `Watch()`，Watch 使用 Consul blocking query 长轮询

## config/source/nacos — Nacos 配置源

```go
import (
    nacosSrc "github.com/Tsukikage7/servex/config/source/nacos"
    "github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
)

// 创建 Nacos 配置源（需要已有的 Nacos 客户端）
src := nacosSrc.New(nacosClient, "myapp.yaml",
    nacosSrc.WithFormat("yaml"),              // 配置格式：json（默认）/ yaml / toml
    nacosSrc.WithGroup("DEFAULT_GROUP"),       // Nacos 分组，默认 "DEFAULT_GROUP"
    nacosSrc.WithNamespace("public"),          // Nacos 命名空间
)

// 与 config.Manager 配合使用
mgr, err := config.NewManager[AppConfig](
    config.WithSource[AppConfig](src),
    config.WithObserver[AppConfig](func(old, new *AppConfig) {
        fmt.Println("Nacos 配置已变更")
    }),
)
// Watch() 基于 Nacos ListenConfig 实现变更监听
```

**关键类型：**
- `nacosSrc.New(client, dataID, opts...)` — 创建 Nacos 配置源
- `WithFormat(format)` — 指定格式（json/yaml/toml），默认 json
- `WithGroup(group)` — 指定 Nacos 分组，默认 "DEFAULT_GROUP"
- `WithNamespace(namespace)` — 指定 Nacos 命名空间
- 支持 `Load()` 和 `Watch()`，Watch 基于 Nacos `ListenConfig` 回调

## config/source/apollo — Apollo 配置中心配置源

```go
import apolloSrc "github.com/Tsukikage7/servex/config/source/apollo"

// 创建 Apollo 配置源（自动启动 Apollo 客户端）
src, err := apolloSrc.New(&apolloSrc.Config{
    Addr:      "http://localhost:8080",  // Apollo 配置服务地址
    AppID:     "myapp",                  // 应用 ID
    Cluster:   "default",               // 集群名，默认 "default"
    Namespace: "application",            // 命名空间，默认 "application"
    Secret:    "",                       // 访问密钥，可选
},
    apolloSrc.WithFormat("yaml"),        // 配置格式：json（默认）/ yaml / toml
)
if err != nil { ... }

// 与 config.Manager 配合使用
mgr, err := config.NewManager[AppConfig](
    config.WithSource[AppConfig](src),
    config.WithObserver[AppConfig](func(old, new *AppConfig) {
        fmt.Println("Apollo 配置已变更")
    }),
)
// Watch() 基于 Apollo ChangeListener 实现变更监听
```

**关键类型：**
- `apolloSrc.New(cfg, opts...)` — 创建 Apollo 配置源，返回 `(*Source, error)`
- `apolloSrc.Config` — 连接配置（`Addr`, `AppID`, `Cluster`, `Namespace`, `Secret`）
- `WithFormat(format)` — 指定格式（json/yaml/toml），默认 json
- `WithCluster(cluster)` — 指定 Apollo 集群，默认 "default"
- `WithNamespace(namespace)` — 指定 Apollo 命名空间，默认 "application"
- 非 properties 格式的命名空间（yaml/json），Apollo 将完整内容存储在 "content" 键下
- 支持 `Load()` 和 `Watch()`，Watch 基于 Apollo `ChangeListener` 回调

## config/source/k8s — Kubernetes ConfigMap/Secret 配置源

```go
import k8sSrc "github.com/Tsukikage7/servex/config/source/k8s"

// 方式一：通过 Config 创建（自动构建 clientset）
src, err := k8sSrc.New(&k8sSrc.Config{
    KubeconfigPath: "",              // 为空则使用 in-cluster 配置
    Namespace:      "production",    // Kubernetes 命名空间，默认 "default"
    Name:           "myapp-config",  // ConfigMap 或 Secret 名称
    ResourceType:   k8sSrc.ResourceConfigMap, // ResourceConfigMap（默认）或 ResourceSecret
    Key:            "config.yaml",   // 指定 data 中的键名，为空则返回所有键值对
    Format:         "yaml",          // 配置格式：json（默认）/ yaml / toml
})
if err != nil { ... }

// 方式二：使用已有的 kubernetes.Interface（便于测试）
src := k8sSrc.NewWithClient(clientset, "myapp-config",
    k8sSrc.WithNamespace("production"),
    k8sSrc.WithResourceType(k8sSrc.ResourceConfigMap),
    k8sSrc.WithKey("config.yaml"),
    k8sSrc.WithFormat("yaml"),
)

// 与 config.Manager 配合使用
mgr, err := config.NewManager[AppConfig](
    config.WithSource[AppConfig](src),
    config.WithObserver[AppConfig](func(old, new *AppConfig) {
        fmt.Println("K8s 配置已变更")
    }),
)
// Watch() 基于 Kubernetes Watch API 实现变更监听

// 从 Secret 读取敏感配置
secretSrc, err := k8sSrc.New(&k8sSrc.Config{
    Name:         "myapp-secret",
    Namespace:    "production",
    ResourceType: k8sSrc.ResourceSecret,
    Key:          "credentials.json",
    Format:       "json",
})
```

**关键类型：**
- `k8sSrc.New(cfg, opts...)` — 通过 Config 创建配置源，返回 `(*Source, error)`
- `k8sSrc.NewWithClient(clientset, name, opts...)` — 使用已有客户端创建配置源
- `k8sSrc.Config` — 连接配置（`KubeconfigPath`, `Namespace`, `Name`, `ResourceType`, `Key`, `Format`）
- `WithFormat(format)` — 指定格式（json/yaml/toml），默认 json
- `WithNamespace(namespace)` — 指定命名空间，默认 "default"
- `WithKey(key)` — 指定 data 中的具体键名，为空则返回所有键值对
- `WithResourceType(rt)` — 指定资源类型（`ResourceConfigMap` 或 `ResourceSecret`）
- 支持 `Load()` 和 `Watch()`，Watch 基于 Kubernetes Watch API

## discovery — 服务注册与发现

```go
// 创建服务发现实例（支持 consul、etcd、静态地址）
disc, err := discovery.NewDiscovery(discovery.Config{
    Type:      discovery.TypeConsul,
    Endpoints: []string{"localhost:8500"},
})
if err != nil { ... }
defer disc.Close()

// 注册服务
id, err := disc.Register(ctx, "my-service", "localhost:8080")

// 注销服务（程序退出前调用）
defer disc.Unregister(ctx, id)

// 服务注册表（封装注册/注销生命周期）
registry, err := discovery.NewServiceRegistry(disc, discovery.ServiceConfig{
    Name: "my-service",
    Addr: "localhost:8080",
})
if err != nil { ... }
defer registry.Close()

// 服务发现（用于 httpclient）
addrs, err := disc.Discover(ctx, "order-service")
```

**关键选项：**
- `discovery.NewDiscovery` — 创建发现客户端（不是 `NewRegistry`）
- `discovery.NewServiceRegistry` — 封装注册生命周期（不是 `NewRegistry`）
- `discovery.TypeConsul` / `TypeEtcd` — 后端类型常量

**静态地址（无注册中心）：**

```go
// 实现 discovery.Discovery 接口，返回硬编码地址
type staticDiscovery struct{ addrs []string }
func (s *staticDiscovery) Discover(_ context.Context, _ string) ([]string, error) {
    return s.addrs, nil
}
// ... 其余方法返回 nil
var _ discovery.Discovery = (*staticDiscovery)(nil)
```
