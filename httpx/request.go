// Package httpx 提供 HTTP 请求上下文提取的统一入口.
//
// 本包整合了多个请求上下文提取模块:
//   - clientip: 客户端 IP 提取、地理位置、访问控制
//   - useragent: User-Agent 解析
//   - deviceinfo: 设备信息Client Hints 优先
//   - botdetect: 机器人检测
//   - locale: 语言区域设置
//   - referer: 来源页面解析
//   - activity: 用户活动追踪
//
// 使用示例:
//
//	// 单独使用子模块
//	import "github.com/Tsukikage7/servex/v2/httpx/clientip"
//	handler = clientip.HTTPMiddleware()(handler)
//
//	// 使用组合中间件
//	import "github.com/Tsukikage7/servex/v2/httpx"
//	handler = httpx.HTTPMiddleware()(handler)
package httpx

import (
	"context"
	"net/http"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/v2/httpx/botdetect"
	"github.com/Tsukikage7/servex/v2/httpx/clientip"
	"github.com/Tsukikage7/servex/v2/httpx/deviceinfo"
	"github.com/Tsukikage7/servex/v2/httpx/locale"
	"github.com/Tsukikage7/servex/v2/httpx/referer"
	"github.com/Tsukikage7/servex/v2/httpx/useragent"
)

// Info 聚合的请求上下文信息.
type Info struct {
	IP        *clientip.IP
	GeoInfo   *clientip.GeoInfo
	UserAgent *useragent.UserAgent
	Device    *deviceinfo.Info
	Bot       *botdetect.Result
	Locale    *locale.Locale
	Referer   *referer.Referer
}

// FromContext 从 context 中提取聚合的请求信息.
func FromContext(ctx context.Context) *Info {
	info := &Info{}

	if ip, ok := clientip.FromContext(ctx); ok {
		info.IP = ip
	}
	if geo, ok := clientip.GeoInfoFromContext(ctx); ok {
		info.GeoInfo = geo
	}
	if ua, ok := useragent.FromContext(ctx); ok {
		info.UserAgent = ua
	}
	if dev, ok := deviceinfo.FromContext(ctx); ok {
		info.Device = dev
	}
	if bot, ok := botdetect.FromContext(ctx); ok {
		info.Bot = bot
	}
	if loc, ok := locale.FromContext(ctx); ok {
		info.Locale = loc
	}
	if ref, ok := referer.FromContext(ctx); ok {
		info.Referer = ref
	}

	return info
}

// Option 中间件配置选项.
type Option func(*options)

type options struct {
	enableClientIP  bool
	enableUserAgent bool
	enableDevice    bool
	enableBot       bool
	enableLocale    bool
	enableReferer   bool

	clientIPOptions []clientip.Option
	deviceOptions   []deviceinfo.Option
	botOptions      []botdetect.Option
	refererOptions  []referer.Option
}

func defaultOptions() *options {
	return &options{
		enableClientIP:  true,
		enableUserAgent: true,
		enableDevice:    false, // 默认关闭，需要额外解析
		enableBot:       false, // 默认关闭，需要额外解析
		enableLocale:    true,
		enableReferer:   true,
	}
}

// WithClientIP 启用客户端 IP 提取.
func WithClientIP(opts ...clientip.Option) Option {
	return func(o *options) {
		o.enableClientIP = true
		o.clientIPOptions = opts
	}
}

// WithUserAgent 启用 User-Agent 解析.
func WithUserAgent() Option {
	return func(o *options) {
		o.enableUserAgent = true
	}
}

// WithDevice 启用设备信息解析.
func WithDevice(opts ...deviceinfo.Option) Option {
	return func(o *options) {
		o.enableDevice = true
		o.deviceOptions = opts
	}
}

// WithBot 启用机器人检测.
func WithBot(opts ...botdetect.Option) Option {
	return func(o *options) {
		o.enableBot = true
		o.botOptions = opts
	}
}

// WithLocale 启用语言区域解析.
func WithLocale() Option {
	return func(o *options) {
		o.enableLocale = true
	}
}

// WithReferer 启用来源页面解析.
func WithReferer(opts ...referer.Option) Option {
	return func(o *options) {
		o.enableReferer = true
		o.refererOptions = opts
	}
}

// WithAll 启用所有解析器.
func WithAll() Option {
	return func(o *options) {
		o.enableClientIP = true
		o.enableUserAgent = true
		o.enableDevice = true
		o.enableBot = true
		o.enableLocale = true
		o.enableReferer = true
	}
}

// DisableClientIP 禁用客户端 IP 提取.
func DisableClientIP() Option {
	return func(o *options) {
		o.enableClientIP = false
	}
}

// DisableUserAgent 禁用 User-Agent 解析.
func DisableUserAgent() Option {
	return func(o *options) {
		o.enableUserAgent = false
	}
}

// DisableLocale 禁用语言区域解析.
func DisableLocale() Option {
	return func(o *options) {
		o.enableLocale = false
	}
}

// DisableReferer 禁用来源页面解析.
func DisableReferer() Option {
	return func(o *options) {
		o.enableReferer = false
	}
}

// HTTPMiddleware 返回组合 HTTP 中间件.
//
// 默认启用: ClientIP, UserAgent, Locale, Referer
// 可选启用: Device, Bot (通过 WithDevice, WithBot 选项)
//
// 示例:
//
//	// 使用默认配置
//	handler = httpx.HTTPMiddleware()(handler)
//
//	// 启用所有解析器
//	handler = httpx.HTTPMiddleware(httpx.WithAll())(handler)
//
//	// 自定义配置
//	handler = httpx.HTTPMiddleware(
//	    httpx.WithClientIP(clientip.WithTrustedProxies("10.0.0.0/8")),
//	    httpx.WithBot(),
//	)(handler)
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(next http.Handler) http.Handler {
		// 按从内到外的顺序构建中间件链
		handler := next

		if o.enableReferer {
			handler = referer.HTTPMiddleware(o.refererOptions...)(handler)
		}
		if o.enableLocale {
			handler = locale.HTTPMiddleware()(handler)
		}
		if o.enableBot {
			handler = botdetect.HTTPMiddleware(o.botOptions...)(handler)
		}
		if o.enableDevice {
			handler = deviceinfo.HTTPMiddleware(o.deviceOptions...)(handler)
		}
		if o.enableUserAgent {
			handler = useragent.HTTPMiddleware()(handler)
		}
		if o.enableClientIP {
			handler = clientip.HTTPMiddleware(o.clientIPOptions...)(handler)
		}

		return handler
	}
}

// UnaryServerInterceptor 返回组合 gRPC 一元拦截器.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// 收集需要的拦截器
	var interceptors []grpc.UnaryServerInterceptor

	if o.enableClientIP {
		interceptors = append(interceptors, clientip.UnaryServerInterceptor(o.clientIPOptions...))
	}
	if o.enableUserAgent {
		interceptors = append(interceptors, useragent.UnaryServerInterceptor())
	}
	if o.enableDevice {
		interceptors = append(interceptors, deviceinfo.UnaryServerInterceptor(o.deviceOptions...))
	}
	if o.enableBot {
		interceptors = append(interceptors, botdetect.UnaryServerInterceptor(o.botOptions...))
	}
	if o.enableLocale {
		interceptors = append(interceptors, locale.UnaryServerInterceptor())
	}
	if o.enableReferer {
		// referer gRPC 拦截器使用 GRPCOption，此处使用默认配置
		interceptors = append(interceptors, referer.UnaryServerInterceptor())
	}

	// 组合拦截器
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 按顺序应用拦截器
		h := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			currentHandler := h
			h = func(ctx context.Context, req any) (any, error) {
				return interceptor(ctx, req, info, currentHandler)
			}
		}
		return h(ctx, req)
	}
}

// StreamServerInterceptor 返回组合 gRPC 流拦截器.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// 收集需要的拦截器
	var interceptors []grpc.StreamServerInterceptor

	if o.enableClientIP {
		interceptors = append(interceptors, clientip.StreamServerInterceptor(o.clientIPOptions...))
	}
	if o.enableUserAgent {
		interceptors = append(interceptors, useragent.StreamServerInterceptor())
	}
	if o.enableDevice {
		interceptors = append(interceptors, deviceinfo.StreamServerInterceptor(o.deviceOptions...))
	}
	if o.enableBot {
		interceptors = append(interceptors, botdetect.StreamServerInterceptor(o.botOptions...))
	}
	if o.enableLocale {
		interceptors = append(interceptors, locale.StreamServerInterceptor())
	}
	if o.enableReferer {
		// referer gRPC 拦截器使用 GRPCOption，此处使用默认配置
		interceptors = append(interceptors, referer.StreamServerInterceptor())
	}

	// 组合拦截器
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 按顺序应用拦截器
		h := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			currentHandler := h
			h = func(srv any, ss grpc.ServerStream) error {
				return interceptor(srv, ss, info, currentHandler)
			}
		}
		return h(srv, ss)
	}
}
