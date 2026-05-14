# request

`github.com/Tsukikage7/servex/v2/httpx`

请求上下文信息提取的组合层，整合多个子模块，提供统一的 HTTP 中间件和 gRPC 拦截器。

## 功能特性

- 聚合多个请求上下文提取模块的结果
- 统一的 HTTP 中间件和 gRPC 一元/流拦截器
- 灵活的选项配置，可按需启用或禁用各子模块
- 默认启用 ClientIP、UserAgent、Locale、Referer

### 子包

| 子包 | 说明 |
|------|------|
| `clientip` | 客户端 IP 提取 |
| `useragent` | User-Agent 解析 |
| `deviceinfo` | 设备信息（Client Hints） |
| `botdetect` | 机器人检测 |
| `locale` | 语言区域设置 |
| `referer` | 来源页面解析 |
| `activity` | 用户活动追踪 |

## API

### Info 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `IP` | `*clientip.IP` | 客户端 IP 信息 |
| `GeoInfo` | `*clientip.GeoInfo` | 地理位置信息 |
| `UserAgent` | `*useragent.UserAgent` | User-Agent 解析结果 |
| `Device` | `*deviceinfo.Info` | 设备信息 |
| `Bot` | `*botdetect.Result` | 机器人检测结果 |
| `Locale` | `*locale.Locale` | 语言区域信息 |
| `Referer` | `*referer.Referer` | 来源页面信息 |

### 函数

| 函数 | 说明 |
|------|------|
| `FromContext(ctx) *Info` | 从 context 提取聚合的请求信息 |
| `HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler` | 返回组合 HTTP 中间件 |
| `UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor` | 返回组合 gRPC 一元拦截器 |
| `StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor` | 返回组合 gRPC 流拦截器 |

### 配置选项

| 选项 | 说明 |
|------|------|
| `WithClientIP(opts ...clientip.Option)` | 启用客户端 IP 提取 |
| `WithUserAgent()` | 启用 User-Agent 解析 |
| `WithDevice(opts ...deviceinfo.Option)` | 启用设备信息解析 |
| `WithBot(opts ...botdetect.Option)` | 启用机器人检测 |
| `WithLocale()` | 启用语言区域解析 |
| `WithReferer(opts ...referer.Option)` | 启用来源页面解析 |
| `WithAll()` | 启用所有解析器 |
| `DisableClientIP()` | 禁用客户端 IP 提取 |
| `DisableUserAgent()` | 禁用 User-Agent 解析 |
| `DisableLocale()` | 禁用语言区域解析 |
| `DisableReferer()` | 禁用来源页面解析 |
