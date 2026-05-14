# botdetect

`github.com/Tsukikage7/servex/v2/httpx/botdetect`

机器人/爬虫检测，基于 User-Agent 模式匹配，内置已知机器人数据库，支持分类与置信度评分。

## 功能特性

- 内置已知机器人数据库（搜索引擎、社交媒体、监控、开发工具等）
- 通用模式匹配检测未知机器人
- 十种分类和三种意图标记
- 置信度评分（0.0 - 1.0）
- HTTP 中间件和 gRPC 拦截器

## API

### Result 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `IsBot` | `bool` | 是否为机器人 |
| `Category` | `Category` | 机器人分类 |
| `Intent` | `Intent` | 机器人意图 |
| `Name` | `string` | 机器人名称 |
| `Company` | `string` | 所属公司/组织 |
| `URL` | `string` | 机器人信息页面 |
| `Confidence` | `float64` | 置信度（0.0 - 1.0） |
| `Reasons` | `[]string` | 检测原因列表 |

### Category 常量

| 常量 | 值 | 说明 |
|------|------|------|
| `CategoryHuman` | `"human"` | 人类用户 |
| `CategorySearch` | `"search"` | 搜索引擎 |
| `CategorySocial` | `"social"` | 社交媒体 |
| `CategoryMonitor` | `"monitor"` | 监控/健康检查 |
| `CategoryFeed` | `"feed"` | RSS/Feed 读取器 |
| `CategoryScraper` | `"scraper"` | 爬虫/抓取器 |
| `CategorySpam` | `"spam"` | 垃圾机器人 |
| `CategorySecurity` | `"security"` | 安全扫描 |
| `CategoryTool` | `"tool"` | 开发/测试工具 |
| `CategoryUnknown` | `"unknown"` | 未知 |

### Intent 常量

| 常量 | 值 | 说明 |
|------|------|------|
| `IntentGood` | `"good"` | 良性机器人 |
| `IntentBad` | `"bad"` | 恶意机器人 |
| `IntentNeutral` | `"neutral"` | 中性/未知意图 |

### Detector

| 方法 | 说明 |
|------|------|
| `New(opts ...Option) *Detector` | 创建检测器 |
| `(d *Detector) Detect(userAgent string) *Result` | 检测 User-Agent |

### Result 方法

| 方法 | 说明 |
|------|------|
| `IsGoodBot() bool` | 是否为良性机器人 |
| `IsBadBot() bool` | 是否为恶意机器人 |

### 函数

| 函数 | 说明 |
|------|------|
| `WithResult(ctx, result) context.Context` | 将检测结果存入 context |
| `FromContext(ctx) (*Result, bool)` | 从 context 获取检测结果 |
| `IsBot(ctx) bool` | 从 context 检查是否为机器人 |
| `GetBotName(ctx) string` | 从 context 获取机器人名称 |
| `GetCategory(ctx) Category` | 从 context 获取机器人分类 |
