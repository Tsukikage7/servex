# consul

基于 Consul KV 存储的配置源实现，支持配置读取和基于 blocking query 的实时变更监听。

## 功能特性

- **Consul KV 配置加载**：从 Consul Key/Value 存储读取配置数据
- **长轮询监听**：基于 Consul blocking query 实现高效的变更监听，无需轮询间隔
- **数据中心支持**：可指定 Consul 数据中心进行跨数据中心配置读取
- **可配置格式**：支持指定配置值的格式，默认为 JSON
- **优雅停止**：基于 context 取消实现监听器的优雅停止

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/config
```

需要额外安装 Consul 客户端库：

```bash
go get github.com/hashicorp/consul/api
```

## API 参考

### New

```go
func New(client *api.Client, key string, opts ...Option) *Source
```

创建 Consul KV 配置源。`client` 为 Consul 客户端实例，`key` 为 KV 存储中的键路径。

### Option

| 函数 | 说明 |
| --- | --- |
| `WithFormat(format string)` | 指定配置格式，默认为 `"json"` |
| `WithDatacenter(dc string)` | 指定 Consul 数据中心 |

### Source 方法

| 方法 | 说明 |
| --- | --- |
| `Load() ([]*config.KeyValue, error)` | 从 Consul KV 读取配置，键不存在时返回 `config.ErrSourceLoad` |
| `Watch() (config.Watcher, error)` | 创建基于 blocking query 的变更监听器 |

### Watcher 方法

| 方法 | 说明 |
| --- | --- |
| `Next() ([]*config.KeyValue, error)` | 阻塞直到 KV 值变更，返回最新配置 |
| `Stop() error` | 取消 context 停止监听 |

## 注意事项

- `Watch()` 使用 Consul 的 blocking query（长轮询）机制，Consul 服务端在值未变更时会保持连接挂起，直到值变更或超时后才返回，比定时轮询更高效。
- 调用 `Stop()` 会取消内部 context，使 `Next()` 返回 `config.ErrSourceClosed` 错误。
- `Load()` 在指定的 key 不存在时返回 `config.ErrSourceLoad` 错误，而非返回空结果。
- 监听器通过 `lastIndex` 跟踪 Consul 的修改索引，确保只在值实际变更时才返回新数据。
- 默认配置格式为 `"json"`，如果 Consul 中存储的是其他格式（如 YAML），需要通过 `WithFormat` 显式指定。
