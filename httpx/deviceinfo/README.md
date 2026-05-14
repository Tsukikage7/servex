# deviceinfo

`github.com/Tsukikage7/servex/v2/httpx/deviceinfo`

设备信息检测，优先使用 Client Hints（Sec-CH-UA-*）获取设备信息，支持 User-Agent 回退。

## 功能特性

- 优先解析 Client Hints 请求头，获取更准确的设备信息
- 支持 User-Agent 回退解析
- 检测移动设备、平台、浏览器、CPU 架构、设备型号
- 解析 Device-Memory、Viewport-Width、DPR 等设备指标
- 提供 Accept-CH / Critical-CH 响应头生成
- HTTP 中间件和 gRPC 拦截器

## API

### Info 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `IsMobile` | `bool` | 是否为移动设备 |
| `Platform` | `string` | 操作系统平台 |
| `PlatformVersion` | `string` | 平台版本 |
| `Browser` | `string` | 浏览器名称 |
| `BrowserVersion` | `string` | 浏览器版本 |
| `Architecture` | `string` | CPU 架构 |
| `Model` | `string` | 设备型号 |
| `Bitness` | `string` | CPU 位数（32/64） |
| `DeviceMemory` | `float64` | 设备内存（GB） |
| `ViewportWidth` | `int` | 视口宽度 |
| `DPR` | `float64` | 设备像素比 |
| `Source` | `DataSource` | 数据来源 |

### DataSource 常量

| 常量 | 值 | 说明 |
|------|------|------|
| `SourceClientHints` | `"client-hints"` | 来自 Client Hints |
| `SourceUserAgent` | `"user-agent"` | 来自 User-Agent 回退 |
| `SourceUnknown` | `"unknown"` | 未知来源 |

### Parser

| 方法 | 说明 |
|------|------|
| `New(opts ...Option) *Parser` | 创建解析器 |
| `(p *Parser) Parse(headers Headers) *Info` | 解析设备信息 |

### Info 方法

| 方法 | 说明 |
|------|------|
| `IsDesktop() bool` | 是否为桌面设备 |
| `IsHighDPI() bool` | 是否为高 DPI 设备（DPR > 1.0） |
| `IsLowMemory() bool` | 是否为低内存设备（< 4GB） |

### 函数

| 函数 | 说明 |
|------|------|
| `AcceptCHHeader() string` | 返回建议的 Accept-CH 响应头值 |
| `CriticalCHHeader() string` | 返回关键 Client Hints 头值 |
| `WithInfo(ctx, info) context.Context` | 将设备信息存入 context |
| `FromContext(ctx) (*Info, bool)` | 从 context 获取设备信息 |
| `IsMobile(ctx) bool` | 从 context 检查是否为移动设备 |
| `GetPlatform(ctx) string` | 从 context 获取平台名称 |
| `GetBrowser(ctx) string` | 从 context 获取浏览器名称 |
