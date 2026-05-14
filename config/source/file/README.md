# file

基于本地文件系统的配置源实现，支持文件读取和实时变更监听。

## 功能特性

- **文件配置加载**：从本地文件读取配置数据
- **格式自动推断**：根据文件扩展名自动推断配置格式（yaml/yml/json/toml）
- **实时监听**：基于 `fsnotify` 监听文件变更，支持写入、创建、删除事件
- **去抖处理**：内置 100ms 去抖机制，合并连续写入事件（如编辑器的 rename+create 操作）
- **可选格式覆盖**：支持通过 `WithFormat` 显式指定配置格式

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/config
```

## API 参考

### New

```go
func New(path string, opts ...Option) *Source
```

创建文件配置源。`path` 为配置文件路径。

### Option

| 函数 | 说明 |
| --- | --- |
| `WithFormat(format string)` | 显式指定配置格式，不使用扩展名推断 |

### Source 方法

| 方法 | 说明 |
| --- | --- |
| `Load() ([]*config.KeyValue, error)` | 读取文件内容，返回配置键值对 |
| `Watch() (config.Watcher, error)` | 创建文件变更监听器 |

### Watcher 方法

| 方法 | 说明 |
| --- | --- |
| `Next() ([]*config.KeyValue, error)` | 阻塞直到文件变更，返回最新配置 |
| `Stop() error` | 停止监听，释放资源 |

### 格式推断规则

| 扩展名 | 推断格式 |
| --- | --- |
| `.yaml`, `.yml` | `yaml` |
| `.json` | `json` |
| `.toml` | `toml` |
| 其他 | 空字符串（需通过 `WithFormat` 指定） |

## 注意事项

- 监听器监听的是文件所在目录而非文件本身，因为部分编辑器保存文件时采用 rename+create 的方式而非直接写入。
- `Next()` 内置 100ms 去抖机制，避免一次保存操作触发多次变更通知。
- 调用 `Stop()` 后，`Next()` 会返回 `config.ErrSourceClosed` 错误。
- `Load()` 返回的 `KeyValue.Key` 为文件路径，`KeyValue.Value` 为文件内容的原始字节。
