// Package deviceinfo 提供设备信息检测功能.
//
// 特性：
//   - 解析 Client Hints 头Sec-CH-UA-*
//   - 支持旧版 User-Agent 解析回退
//   - 设备类型、平台、浏览器检测
//   - HTTP/gRPC 中间件支持
//   - 将解析结果存入 context 供链路使用
//
// Client Hints 是现代浏览器提供的更可靠的设备信息获取方式，
// 服务端需要发送 Accept-CH 响应头来请求这些信息。
//
// 示例：
//
//	handler = deviceinfo.HTTPMiddleware()(handler)
//
//	info := deviceinfo.FromContext(ctx)
//	fmt.Println(info.IsMobile)     // true
//	fmt.Println(info.Platform)     // "Android"
//	fmt.Println(info.Browser)      // "Chrome"
package deviceinfo

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// contextKey context 键类型.
type contextKey string

const (
	deviceContextKey contextKey = "deviceinfo:info"
)

// Info 设备信息.
type Info struct {
	// IsMobile 是否为移动设备
	IsMobile bool

	// Platform 操作系统平台
	Platform string

	// PlatformVersion 平台版本
	PlatformVersion string

	// Browser 浏览器名称
	Browser string

	// BrowserVersion 浏览器版本
	BrowserVersion string

	// Architecture CPU 架构
	Architecture string

	// Model 设备型号
	Model string

	// Bitness CPU 位数 (32/64)
	Bitness string

	// DeviceMemory 设备内存 (GB)
	DeviceMemory float64

	// ViewportWidth 视口宽度
	ViewportWidth int

	// DPR 设备像素比
	DPR float64

	// Source 数据来源
	Source DataSource
}

// DataSource 数据来源.
type DataSource string

const (
	SourceClientHints DataSource = "client-hints" // 来自 Client Hints
	SourceUserAgent   DataSource = "user-agent"   // 来自 User-Agent
	SourceUnknown     DataSource = "unknown"      // 未知
)

// IsDesktop 是否为桌面设备.
func (i *Info) IsDesktop() bool {
	return !i.IsMobile
}

// IsHighDPI 是否为高 DPI 设备.
func (i *Info) IsHighDPI() bool {
	return i.DPR > 1.0
}

// IsLowMemory 是否为低内存设备 (< 4GB).
func (i *Info) IsLowMemory() bool {
	return i.DeviceMemory > 0 && i.DeviceMemory < 4
}

// WithInfo 将设备信息存入 context.
func WithInfo(ctx context.Context, info *Info) context.Context {
	return context.WithValue(ctx, deviceContextKey, info)
}

// FromContext 从 context 获取设备信息.
func FromContext(ctx context.Context) (*Info, bool) {
	info, ok := ctx.Value(deviceContextKey).(*Info)
	return info, ok
}

// IsMobile 从 context 检查是否为移动设备.
func IsMobile(ctx context.Context) bool {
	if info, ok := FromContext(ctx); ok {
		return info.IsMobile
	}
	return false
}

// GetPlatform 从 context 获取平台名称.
func GetPlatform(ctx context.Context) string {
	if info, ok := FromContext(ctx); ok {
		return info.Platform
	}
	return ""
}

// GetBrowser 从 context 获取浏览器名称.
func GetBrowser(ctx context.Context) string {
	if info, ok := FromContext(ctx); ok {
		return info.Browser
	}
	return ""
}

// Parser 设备信息解析器.
type Parser struct {
	opts *options
}

// New 创建设备信息解析器.
func New(opts ...Option) *Parser {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Parser{opts: o}
}

// Headers 请求头集合.
type Headers struct {
	// Client Hints
	SecCHUA                string // Sec-CH-UA
	SecCHUAMobile          string // Sec-CH-UA-Mobile
	SecCHUAPlatform        string // Sec-CH-UA-Platform
	SecCHUAPlatformVersion string // Sec-CH-UA-Platform-Version
	SecCHUAArch            string // Sec-CH-UA-Arch
	SecCHUAModel           string // Sec-CH-UA-Model
	SecCHUABitness         string // Sec-CH-UA-Bitness
	SecCHUAFullVersionList string // Sec-CH-UA-Full-Version-List

	// Device hints
	DeviceMemory  string // Device-Memory
	ViewportWidth string // Viewport-Width
	DPR           string // DPR

	// Fallback
	UserAgent string // User-Agent
}

// Parse 解析设备信息.
func (p *Parser) Parse(headers Headers) *Info {
	info := &Info{
		Source: SourceUnknown,
	}

	// 优先使用 Client Hints
	if hasClientHints(headers) {
		info.Source = SourceClientHints
		parseClientHints(headers, info)
	} else if headers.UserAgent != "" && p.opts.enableUAFallback {
		// 回退到 User-Agent
		info.Source = SourceUserAgent
		parseUserAgent(headers.UserAgent, info)
	}

	// 解析额外的设备 hints
	parseDeviceHints(headers, info)

	return info
}

// hasClientHints 检查是否有 Client Hints.
func hasClientHints(h Headers) bool {
	return h.SecCHUA != "" || h.SecCHUAMobile != "" || h.SecCHUAPlatform != ""
}

// parseClientHints 解析 Client Hints.
func parseClientHints(h Headers, info *Info) {
	// Sec-CH-UA-Mobile
	info.IsMobile = h.SecCHUAMobile == "?1"

	// Sec-CH-UA-Platform
	info.Platform = trimQuotes(h.SecCHUAPlatform)

	// Sec-CH-UA-Platform-Version
	info.PlatformVersion = trimQuotes(h.SecCHUAPlatformVersion)

	// Sec-CH-UA-Arch
	info.Architecture = trimQuotes(h.SecCHUAArch)

	// Sec-CH-UA-Model
	info.Model = trimQuotes(h.SecCHUAModel)

	// Sec-CH-UA-Bitness
	info.Bitness = trimQuotes(h.SecCHUABitness)

	// Sec-CH-UA 或 Sec-CH-UA-Full-Version-List 解析浏览器
	uaList := h.SecCHUAFullVersionList
	if uaList == "" {
		uaList = h.SecCHUA
	}
	if uaList != "" {
		info.Browser, info.BrowserVersion = parseBrandList(uaList)
	}
}

// parseBrandList 解析浏览器品牌列表.
// 格式: "Chromium";v="120", "Google Chrome";v="120", "Not-A.Brand";v="24"
func parseBrandList(list string) (browser, version string) {
	// 正则匹配品牌和版本
	re := regexp.MustCompile(`"([^"]+)";v="([^"]+)"`)
	matches := re.FindAllStringSubmatch(list, -1)

	// 优先级：Chrome > Edge > Firefox > Safari > Chromium > 其他
	priority := map[string]int{
		"Google Chrome":  1,
		"Microsoft Edge": 2,
		"Firefox":        3,
		"Safari":         4,
		"Chromium":       5,
	}

	var bestBrowser, bestVersion string
	bestPriority := 999

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		brand := match[1]
		ver := match[2]

		// 跳过 "Not" 品牌
		if strings.HasPrefix(brand, "Not") {
			continue
		}

		p, ok := priority[brand]
		if !ok {
			p = 10 // 其他浏览器
		}

		if p < bestPriority {
			bestPriority = p
			bestBrowser = brand
			bestVersion = ver
		}
	}

	return bestBrowser, bestVersion
}

// parseUserAgent 从 User-Agent 解析回退方案.
func parseUserAgent(ua string, info *Info) {
	uaLower := strings.ToLower(ua)

	// 检测移动设备
	mobilePatterns := []string{"mobile", "android", "iphone", "ipad", "ipod"}
	for _, p := range mobilePatterns {
		if strings.Contains(uaLower, p) {
			info.IsMobile = true
			break
		}
	}

	// Tablet 不算移动设备
	if strings.Contains(uaLower, "ipad") ||
		(strings.Contains(uaLower, "android") && !strings.Contains(uaLower, "mobile")) {
		info.IsMobile = false
	}

	// 检测平台
	switch {
	case strings.Contains(uaLower, "windows"):
		info.Platform = "Windows"
	case strings.Contains(uaLower, "mac os x"):
		if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") {
			info.Platform = "iOS"
		} else {
			info.Platform = "macOS"
		}
	case strings.Contains(uaLower, "android"):
		info.Platform = "Android"
	case strings.Contains(uaLower, "linux"):
		info.Platform = "Linux"
	case strings.Contains(uaLower, "cros"):
		info.Platform = "Chrome OS"
	}

	// 检测浏览器顺序很重要
	switch {
	case strings.Contains(uaLower, "edg/") || strings.Contains(uaLower, "edge/"):
		info.Browser = "Microsoft Edge"
	case strings.Contains(uaLower, "crios"): // Chrome on iOS
		info.Browser = "Google Chrome"
	case strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "chromium"):
		info.Browser = "Google Chrome"
	case strings.Contains(uaLower, "firefox") || strings.Contains(uaLower, "fxios"): // Firefox on iOS
		info.Browser = "Firefox"
	case strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "crios"):
		info.Browser = "Safari"
	}
}

// parseDeviceHints 解析设备 hints.
func parseDeviceHints(h Headers, info *Info) {
	// Device-Memory
	if h.DeviceMemory != "" {
		if mem, err := strconv.ParseFloat(h.DeviceMemory, 64); err == nil {
			info.DeviceMemory = mem
		}
	}

	// Viewport-Width
	if h.ViewportWidth != "" {
		if w, err := strconv.Atoi(h.ViewportWidth); err == nil {
			info.ViewportWidth = w
		}
	}

	// DPR
	if h.DPR != "" {
		if dpr, err := strconv.ParseFloat(h.DPR, 64); err == nil {
			info.DPR = dpr
		}
	}
}

// trimQuotes 移除字符串两端的引号.
func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// AcceptCHHeader 返回建议的 Accept-CH 响应头值.
// 服务端应在响应中发送此头以请求 Client Hints.
func AcceptCHHeader() string {
	return "Sec-CH-UA, Sec-CH-UA-Mobile, Sec-CH-UA-Platform, Sec-CH-UA-Platform-Version, Sec-CH-UA-Arch, Sec-CH-UA-Model, Sec-CH-UA-Full-Version-List, Device-Memory, Viewport-Width, DPR"
}

// CriticalCHHeader 返回关键 Client Hints 头值.
func CriticalCHHeader() string {
	return "Sec-CH-UA-Mobile, Sec-CH-UA-Platform"
}
