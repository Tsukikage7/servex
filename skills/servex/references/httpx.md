# servex HTTP 请求上下文提取

## httpx -- 组合中间件（推荐入口）

```go
import "github.com/Tsukikage7/servex/v2/httpx"

// 使用默认配置（启用 ClientIP + UserAgent + Locale + Referer）
handler = httpx.HTTPMiddleware()(handler)

// 启用全部解析器
handler = httpx.HTTPMiddleware(httpx.WithAll())(handler)

// 自定义配置
handler = httpx.HTTPMiddleware(
    httpx.WithClientIP(clientip.WithTrustedProxies("10.0.0.0/8")),
    httpx.WithBot(),
    httpx.WithDevice(),
    httpx.DisableReferer(),
)(handler)

// 从 context 提取聚合信息
func myHandler(w http.ResponseWriter, r *http.Request) {
    info := httpx.FromContext(r.Context())

    info.IP        // *clientip.IP
    info.GeoInfo   // *clientip.GeoInfo
    info.UserAgent // *useragent.UserAgent
    info.Device    // *deviceinfo.Info
    info.Bot       // *botdetect.Result
    info.Locale    // *locale.Locale
    info.Referer   // *referer.Referer
}
```

**默认启用：** ClientIP, UserAgent, Locale, Referer
**需手动启用：** Device（`WithDevice()`）、Bot（`WithBot()`）
**禁用选项：** `DisableClientIP()`, `DisableUserAgent()`, `DisableLocale()`, `DisableReferer()`

## gRPC 拦截器

```go
import "github.com/Tsukikage7/servex/v2/httpx"

// 一元拦截器
grpcserver.New(
    grpcserver.WithUnaryInterceptor(httpx.UnaryServerInterceptor(httpx.WithAll())),
    grpcserver.WithStreamInterceptor(httpx.StreamServerInterceptor()),
)
```

## clientip -- 客户端 IP

```go
import "github.com/Tsukikage7/servex/v2/httpx/clientip"

// HTTP 中间件
handler = clientip.HTTPMiddleware(
    clientip.WithTrustedProxies("10.0.0.0/8", "172.16.0.0/12"),
)(handler)

// 从 context 获取
ip, ok := clientip.FromContext(ctx)
fmt.Println(ip.String()) // "203.0.113.1"

// 地理位置信息（需配置 GeoIP 数据库）
geo, ok := clientip.GeoInfoFromContext(ctx)
```

## useragent -- User-Agent 解析

```go
import "github.com/Tsukikage7/servex/v2/httpx/useragent"

handler = useragent.HTTPMiddleware()(handler)

ua, ok := useragent.FromContext(ctx)
// ua.Browser, ua.OS, ua.Device 等
```

## deviceinfo -- 设备信息

```go
import "github.com/Tsukikage7/servex/v2/httpx/deviceinfo"

// 优先使用 Client Hints，回退到 User-Agent
handler = deviceinfo.HTTPMiddleware()(handler)

dev, ok := deviceinfo.FromContext(ctx)
```

## botdetect -- 机器人检测

```go
import "github.com/Tsukikage7/servex/v2/httpx/botdetect"

handler = botdetect.HTTPMiddleware()(handler)

bot, ok := botdetect.FromContext(ctx)
// bot.IsBot, bot.Name 等
```

## locale -- 语言区域

```go
import "github.com/Tsukikage7/servex/v2/httpx/locale"

handler = locale.HTTPMiddleware()(handler)

loc, ok := locale.FromContext(ctx)
// loc.Language, loc.Region 等
```

## referer -- 来源页面

```go
import "github.com/Tsukikage7/servex/v2/httpx/referer"

handler = referer.HTTPMiddleware()(handler)

ref, ok := referer.FromContext(ctx)
// ref.URL, ref.Domain 等
```

## activity -- 用户活动追踪

```go
import "github.com/Tsukikage7/servex/v2/httpx/activity"

// 记录用户活动（如页面访问、操作行为）
// 与其他 httpx 子模块配合使用
```
