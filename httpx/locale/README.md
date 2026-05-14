# locale

`github.com/Tsukikage7/servex/v2/httpx/locale`

语言区域设置解析，从 Accept-Language 请求头提取用户的语言偏好和地区信息。

## 功能特性

- 解析 Accept-Language 请求头
- 支持语言标签完整规范：语言代码、地区代码、文字代码、质量值
- 按质量值排序的首选语言列表
- 语言匹配和最佳候选选择
- HTTP 中间件和 gRPC 拦截器

## API

### Locale 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `Raw` | `string` | 原始 Accept-Language 字符串 |
| `Preferred` | `[]Tag` | 首选语言标签列表（按质量值降序） |

### Tag 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `Language` | `string` | 语言代码（如 "zh", "en"） |
| `Region` | `string` | 地区代码（如 "CN", "US"） |
| `Script` | `string` | 文字代码（如 "Hans", "Hant"） |
| `Quality` | `float64` | 质量值（0.0 - 1.0，默认 1.0） |
| `Raw` | `string` | 原始标签字符串 |

### Locale 方法

| 方法 | 说明 |
|------|------|
| `Language() string` | 返回首选语言代码 |
| `Region() string` | 返回首选地区代码 |
| `String() string` | 返回首选语言标签字符串 |
| `Match(languages ...string) bool` | 检查偏好中是否包含指定语言 |
| `Best(candidates ...string) string` | 从候选列表中选择最佳匹配 |

### 函数

| 函数 | 说明 |
|------|------|
| `Parse(raw string) *Locale` | 解析 Accept-Language 字符串 |
| `WithLocale(ctx, loc) context.Context` | 将 Locale 存入 context |
| `FromContext(ctx) (*Locale, bool)` | 从 context 获取 Locale |
| `GetLanguage(ctx) string` | 从 context 获取首选语言代码 |
| `GetRegion(ctx) string` | 从 context 获取首选地区代码 |
| `GetLocale(ctx) string` | 从 context 获取首选语言标签字符串 |
