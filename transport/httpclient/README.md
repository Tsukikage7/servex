# httpclient

`httpclient` 包提供 HTTP 客户端封装，集成服务发现机制，提供便捷的 RESTful 请求方法。

## 功能特性

- 集成服务发现，通过服务名自动解析目标地址
- 提供 Get、Post、Put、Delete 便捷方法
- 支持自定义请求头、超时时间和 Transport
- 自动构建 baseURL，简化路径拼接

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/httpclient
```

## API

### Client

```go
func New(opts ...Option) (*Client, error)
func (c *Client) HTTPClient() *http.Client
func (c *Client) BaseURL() string
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error)
func (c *Client) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error)
func (c *Client) Put(ctx context.Context, path string, body io.Reader) (*http.Response, error)
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error)
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error)
```

`New` 方法会通过服务发现解析目标地址并构建 baseURL。若 `serviceName`、`discovery` 或 `logger` 未设置，将触发 panic。

### 配置选项

| 选项              | 默认值        | 说明                   |
| ----------------- | ------------- | ---------------------- |
| `WithName`        | `HTTP-Client` | 客户端名称（用于日志） |
| `WithServiceName` | -             | 目标服务名称（必需）   |
| `WithDiscovery`   | -             | 服务发现实例（必需）   |
| `WithLogger`      | -             | 日志记录器（必需）     |
| `WithScheme`      | `http`        | URL scheme             |
| `WithTimeout`     | `30s`         | 请求超时时间           |
| `WithHeader`      | -             | 添加单个默认请求头     |
| `WithHeaders`     | -             | 批量设置默认请求头     |
| `WithTransport`   | -             | 自定义 http.RoundTripper |

### 服务发现集成

客户端在创建时通过 `discovery.Discovery` 接口解析服务地址，并自动拼接为 baseURL：

```go
client, err := httpclient.New(
    httpclient.WithServiceName("user-service"),
    httpclient.WithDiscovery(consulDiscovery),
    httpclient.WithLogger(log),
    httpclient.WithScheme("https"),
    httpclient.WithTimeout(10 * time.Second),
    httpclient.WithHeader("X-API-Key", "secret"),
)
```

所有请求方法会自动将 path 拼接到 baseURL 后，并附加通过 `WithHeader`/`WithHeaders` 设置的默认请求头。

### 错误处理

| 错误                 | 说明                 |
| -------------------- | -------------------- |
| `ErrDiscoveryFailed` | 服务发现失败         |
| `ErrServiceNotFound` | 指定服务名未找到实例 |
| `ErrRequestFailed`   | HTTP 请求构建失败    |

## 许可证

详见项目根目录 LICENSE 文件。
