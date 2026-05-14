# referer

`github.com/Tsukikage7/servex/v2/httpx/referer` -- 来源页面与UTM参数解析。

## 概述

referer 包提供 HTTP Referer 头解析功能，支持来源分类、UTM 营销参数提取以及 HTTP/gRPC 中间件集成。解析结果自动存入 context，可在请求链路中随时获取。

## 功能特性

- 解析来源 URL（域名、路径、查询参数）
- 分类来源类型：搜索引擎、社交媒体、直接访问、外部引荐、站内跳转、邮件营销、付费广告
- 提取 UTM 营销追踪参数（source、medium、campaign、term、content）
- HTTP 中间件与 gRPC 拦截器支持
- 内置主流搜索引擎列表（Google、Bing、百度、搜狗、360等）
- 内置主流社交网络列表（微博、知乎、B站、微信、Facebook、Twitter等）

## API

### 类型

| 类型 | 说明 |
|------|------|
| `Referer` | 来源信息，包含 Raw、URL、Type、Source、Domain、Path、SearchQuery、UTM |
| `UTMParams` | UTM 参数，包含 Source、Medium、Campaign、Term、Content |
| `SourceType` | 来源类型枚举 |

### SourceType 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `SourceTypeDirect` | `"direct"` | 直接访问 |
| `SourceTypeSearch` | `"search"` | 搜索引擎 |
| `SourceTypeSocial` | `"social"` | 社交媒体 |
| `SourceTypeReferral` | `"referral"` | 外部引荐 |
| `SourceTypeInternal` | `"internal"` | 站内跳转 |
| `SourceTypeEmail` | `"email"` | 邮件营销 |
| `SourceTypePaid` | `"paid"` | 付费广告 |
| `SourceTypeUnknown` | `"unknown"` | 未知 |

### 函数

| 函数 | 说明 |
|------|------|
| `Parse(raw string) *Referer` | 解析 Referer 字符串 |
| `ParseWithHost(raw, currentHost string) *Referer` | 解析并判断站内/站外 |
| `WithReferer(ctx, ref) context.Context` | 将 Referer 存入 context |
| `FromContext(ctx) (*Referer, bool)` | 从 context 获取 Referer |
| `GetSource(ctx) string` | 从 context 获取来源名称 |
| `GetSourceType(ctx) SourceType` | 从 context 获取来源类型 |
| `GetDomain(ctx) string` | 从 context 获取来源域名 |
| `HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler` | HTTP 中间件 |
| `UnaryServerInterceptor(opts ...GRPCOption) grpc.UnaryServerInterceptor` | gRPC 一元拦截器 |
| `StreamServerInterceptor(opts ...GRPCOption) grpc.StreamServerInterceptor` | gRPC 流拦截器 |
