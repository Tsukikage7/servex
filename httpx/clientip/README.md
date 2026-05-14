# clientip

`github.com/Tsukikage7/servex/v2/httpx/clientip`

客户端 IP 提取与解析，支持从 HTTP 请求和 gRPC 调用中获取真实客户端 IP 地址。

## 功能特性

- 从多种来源提取客户端 IP：RemoteAddr、X-Forwarded-For、X-Real-IP
- 支持 IPv4 和 IPv6 地址解析
- 可信代理感知的 X-Forwarded-For 解析
- 私有 IP 和合法性校验
- HTTP 中间件和 gRPC 拦截器
- Context 存取

## API

### IP 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `Address` | `string` | 纯 IP 地址（不含端口） |
| `Port` | `string` | 端口号（可选） |
| `Raw` | `string` | 原始值 |

### 解析函数

| 函数 | 说明 |
|------|------|
| `ParseIP(addr string) *IP` | 解析 IP 地址字符串，支持 IPv4/IPv6 和端口 |
| `ParseXForwardedFor(xff string) string` | 解析 X-Forwarded-For 头，返回第一个 IP |
| `ParseXForwardedForWithTrust(xff string, isTrusted func(string) bool) string` | 从右向左遍历，跳过可信代理 |
| `IsPrivateIP(ipStr string) bool` | 检查是否为私有/回环/链路本地地址 |
| `IsValidIP(ipStr string) bool` | 检查是否为有效 IP 地址 |

### Context 操作

| 函数 | 说明 |
|------|------|
| `WithIP(ctx, ip *IP) context.Context` | 将 IP 信息存入 context |
| `FromContext(ctx) (*IP, bool)` | 从 context 获取 IP 信息 |
| `GetIP(ctx) string` | 便捷方法，返回 IP 地址字符串 |
| `MustFromContext(ctx) *IP` | 获取 IP 信息，不存在时 panic |
