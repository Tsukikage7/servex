# useragent

`github.com/Tsukikage7/servex/v2/httpx/useragent`

User-Agent 字符串解析，提取浏览器、操作系统、设备类型和渲染引擎等信息。

## 功能特性

- 解析浏览器名称及版本（Chrome、Firefox、Safari、Edge、Opera、IE）
- 解析操作系统及版本（Windows、macOS、Linux、iOS、Android）
- 设备类型识别：Desktop、Mobile、Tablet、Bot
- 设备品牌和型号检测（Apple、Samsung、Huawei、Xiaomi 等）
- 渲染引擎识别（Blink、Gecko、WebKit、Trident）
- HTTP 中间件和 gRPC 拦截器

## API

### UserAgent 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `Raw` | `string` | 原始 User-Agent 字符串 |
| `Browser` | `Browser` | 浏览器信息（Name, Version, Full） |
| `OS` | `OS` | 操作系统信息（Name, Version） |
| `Device` | `Device` | 设备信息（Type, Brand, Model） |
| `Engine` | `Engine` | 渲染引擎信息（Name, Version） |

### DeviceType 常量

| 常量 | 值 | 说明 |
|------|------|------|
| `DeviceTypeDesktop` | `"Desktop"` | 桌面设备 |
| `DeviceTypeMobile` | `"Mobile"` | 移动设备 |
| `DeviceTypeTablet` | `"Tablet"` | 平板设备 |
| `DeviceTypeBot` | `"Bot"` | 机器人/爬虫 |
| `DeviceTypeUnknown` | `"Unknown"` | 未知 |

### 方法

| 方法 | 说明 |
|------|------|
| `(ua *UserAgent) IsMobile() bool` | 是否为移动设备 |
| `(ua *UserAgent) IsTablet() bool` | 是否为平板设备 |
| `(ua *UserAgent) IsDesktop() bool` | 是否为桌面设备 |
| `(ua *UserAgent) IsBot() bool` | 是否为机器人 |

### 函数

| 函数 | 说明 |
|------|------|
| `Parse(raw string) *UserAgent` | 解析 User-Agent 字符串 |
| `WithUserAgent(ctx, ua) context.Context` | 将 UserAgent 存入 context |
| `FromContext(ctx) (*UserAgent, bool)` | 从 context 获取 UserAgent |
| `GetBrowser(ctx) string` | 从 context 获取浏览器名称 |
| `GetOS(ctx) string` | 从 context 获取操作系统名称 |
| `GetDeviceType(ctx) DeviceType` | 从 context 获取设备类型 |
