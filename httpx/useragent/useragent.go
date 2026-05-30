// Package useragent 提供 User-Agent 解析功能.
//
// 特性：
//   - 解析浏览器、操作系统、设备类型
//   - HTTP/gRPC 中间件支持
//   - 将解析结果存入 context 供链路使用
//
// 示例：
//
//	handler = useragent.HTTPMiddleware()(handler)
//
//	ua := useragent.FromContext(ctx)
//	fmt.Println(ua.Browser.Name)    // "Chrome"
//	fmt.Println(ua.OS.Name)         // "Windows"
//	fmt.Println(ua.Device.Type)     // "Desktop"
package useragent

import (
	"context"
	"regexp"
	"strings"
)

// contextKey context 键类型.
type contextKey string

const (
	uaContextKey contextKey = "useragent:ua"
)

// UserAgent 解析后的 User-Agent 信息.
type UserAgent struct {
	// Raw 原始 User-Agent 字符串
	Raw string

	// Browser 浏览器信息
	Browser Browser

	// OS 操作系统信息
	OS OS

	// Device 设备信息
	Device Device

	// Engine 渲染引擎信息
	Engine Engine
}

// Browser 浏览器信息.
type Browser struct {
	Name    string // Chrome, Firefox, Safari, Edge, IE, Opera 等
	Version string // 主版本号
	Full    string // 完整版本号
}

// OS 操作系统信息.
type OS struct {
	Name    string // Windows, macOS, Linux, iOS, Android 等
	Version string // 版本号
}

// Device 设备信息.
type Device struct {
	Type  DeviceType // Desktop, Mobile, Tablet, Bot 等
	Brand string     // Apple, Samsung, Huawei 等
	Model string     // iPhone, Galaxy 等
}

// Engine 渲染引擎信息.
type Engine struct {
	Name    string // Blink, Gecko, WebKit, Trident 等
	Version string
}

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeDesktop DeviceType = "Desktop"
	DeviceTypeMobile  DeviceType = "Mobile"
	DeviceTypeTablet  DeviceType = "Tablet"
	DeviceTypeBot     DeviceType = "Bot"
	DeviceTypeUnknown DeviceType = "Unknown"
)

// IsMobile 是否为移动设备.
func (ua *UserAgent) IsMobile() bool {
	return ua.Device.Type == DeviceTypeMobile
}

// IsTablet 是否为平板设备.
func (ua *UserAgent) IsTablet() bool {
	return ua.Device.Type == DeviceTypeTablet
}

// IsDesktop 是否为桌面设备.
func (ua *UserAgent) IsDesktop() bool {
	return ua.Device.Type == DeviceTypeDesktop
}

// IsBot 是否为机器人/爬虫.
func (ua *UserAgent) IsBot() bool {
	return ua.Device.Type == DeviceTypeBot
}

// String 返回简短描述.
func (ua *UserAgent) String() string {
	if ua == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if ua.Browser.Name != "" {
		parts = append(parts, ua.Browser.Name)
	}
	if ua.OS.Name != "" {
		parts = append(parts, ua.OS.Name)
	}
	if ua.Device.Type != "" {
		parts = append(parts, string(ua.Device.Type))
	}
	return strings.Join(parts, " / ")
}

// WithUserAgent 将 UserAgent 存入 context.
func WithUserAgent(ctx context.Context, ua *UserAgent) context.Context {
	return context.WithValue(ctx, uaContextKey, ua)
}

// FromContext 从 context 获取 UserAgent.
func FromContext(ctx context.Context) (*UserAgent, bool) {
	ua, ok := ctx.Value(uaContextKey).(*UserAgent)
	return ua, ok
}

// GetBrowser 从 context 获取浏览器名称.
func GetBrowser(ctx context.Context) string {
	if ua, ok := FromContext(ctx); ok {
		return ua.Browser.Name
	}
	return ""
}

// GetOS 从 context 获取操作系统名称.
func GetOS(ctx context.Context) string {
	if ua, ok := FromContext(ctx); ok {
		return ua.OS.Name
	}
	return ""
}

// GetDeviceType 从 context 获取设备类型.
func GetDeviceType(ctx context.Context) DeviceType {
	if ua, ok := FromContext(ctx); ok {
		return ua.Device.Type
	}
	return DeviceTypeUnknown
}

// Parse 解析 User-Agent 字符串.
func Parse(raw string) *UserAgent {
	ua := &UserAgent{Raw: raw}
	if raw == "" {
		ua.Device.Type = DeviceTypeDesktop
		return ua
	}

	rawLower := strings.ToLower(raw)

	// 检测设备类型和是否为 Bot
	ua.Device.Type = detectDeviceType(rawLower)

	// 解析操作系统
	ua.OS = parseOS(raw, rawLower)

	// 解析浏览器
	ua.Browser = parseBrowser(raw, rawLower)

	// 解析引擎
	ua.Engine = parseEngine(raw, rawLower)

	// 解析设备品牌和型号
	ua.Device.Brand, ua.Device.Model = parseDevice(raw, rawLower)

	return ua
}

// detectDeviceType 检测设备类型.
func detectDeviceType(rawLower string) DeviceType {
	// 空字符串默认为桌面
	if rawLower == "" {
		return DeviceTypeDesktop
	}

	// Bot 检测
	botPatterns := []string{
		"bot", "spider", "crawler", "scraper", "curl", "wget",
		"python", "java", "go-http", "httpclient", "axios",
		"googlebot", "bingbot", "baiduspider", "yandex", "duckduckbot",
		"facebookexternalhit", "twitterbot", "linkedinbot", "slackbot",
	}
	for _, p := range botPatterns {
		if strings.Contains(rawLower, p) {
			return DeviceTypeBot
		}
	}

	// Tablet 检测要在 Mobile 之前
	if strings.Contains(rawLower, "ipad") ||
		(strings.Contains(rawLower, "android") && !strings.Contains(rawLower, "mobile")) ||
		strings.Contains(rawLower, "tablet") {
		return DeviceTypeTablet
	}

	// Mobile 检测
	mobilePatterns := []string{
		"mobile", "iphone", "ipod", "android", "blackberry",
		"windows phone", "webos", "opera mini", "opera mobi",
	}
	for _, p := range mobilePatterns {
		if strings.Contains(rawLower, p) {
			return DeviceTypeMobile
		}
	}

	// 默认为桌面
	return DeviceTypeDesktop
}

// parseOS 解析操作系统.
func parseOS(raw, rawLower string) OS {
	os := OS{}

	// iOS 检测要在 macOS 之前，因为 iOS UA 也包含 "like Mac OS X"
	switch {
	case strings.Contains(rawLower, "iphone") || strings.Contains(rawLower, "ipad") || strings.Contains(rawLower, "ipod"):
		os.Name = "iOS"
		if match := regexp.MustCompile(`(?i)(?:iPhone OS|CPU (?:iPhone )?OS) (\d+[._]\d+)`).FindStringSubmatch(raw); len(match) > 1 {
			os.Version = strings.ReplaceAll(match[1], "_", ".")
		}
	case strings.Contains(rawLower, "windows nt 10"):
		os.Name = "Windows"
		os.Version = "10"
	case strings.Contains(rawLower, "windows nt 6.3"):
		os.Name = "Windows"
		os.Version = "8.1"
	case strings.Contains(rawLower, "windows nt 6.2"):
		os.Name = "Windows"
		os.Version = "8"
	case strings.Contains(rawLower, "windows nt 6.1"):
		os.Name = "Windows"
		os.Version = "7"
	case strings.Contains(rawLower, "windows"):
		os.Name = "Windows"
	case strings.Contains(rawLower, "mac os x"):
		os.Name = "macOS"
		if match := regexp.MustCompile(`Mac OS X (\d+[._]\d+)`).FindStringSubmatch(raw); len(match) > 1 {
			os.Version = strings.ReplaceAll(match[1], "_", ".")
		}
	case strings.Contains(rawLower, "android"):
		os.Name = "Android"
		if match := regexp.MustCompile(`Android (\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 1 {
			os.Version = match[1]
		}
	case strings.Contains(rawLower, "linux"):
		os.Name = "Linux"
	case strings.Contains(rawLower, "cros"):
		os.Name = "Chrome OS"
	}

	return os
}

// parseBrowser 解析浏览器.
func parseBrowser(raw, rawLower string) Browser {
	browser := Browser{}

	// 顺序很重要，需要先检测特定浏览器
	switch {
	case strings.Contains(rawLower, "edg/") || strings.Contains(rawLower, "edge/"):
		browser.Name = "Edge"
		if match := regexp.MustCompile(`Edg(?:e)?/(\d+)\.(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 2 {
			browser.Version = match[1]
			browser.Full = match[1] + "." + match[2]
		}
	case strings.Contains(rawLower, "opr/") || strings.Contains(rawLower, "opera"):
		browser.Name = "Opera"
		if match := regexp.MustCompile(`(?:OPR|Opera)/(\d+)\.(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 2 {
			browser.Version = match[1]
			browser.Full = match[1] + "." + match[2]
		}
	case strings.Contains(rawLower, "chrome") && !strings.Contains(rawLower, "chromium"):
		browser.Name = "Chrome"
		if match := regexp.MustCompile(`Chrome/(\d+)\.(\d+\.?\d*\.?\d*)`).FindStringSubmatch(raw); len(match) > 2 {
			browser.Version = match[1]
			browser.Full = match[1] + "." + match[2]
		}
	case strings.Contains(rawLower, "firefox"):
		browser.Name = "Firefox"
		if match := regexp.MustCompile(`Firefox/(\d+)\.(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 2 {
			browser.Version = match[1]
			browser.Full = match[1] + "." + match[2]
		}
	case strings.Contains(rawLower, "safari") && !strings.Contains(rawLower, "chrome"):
		browser.Name = "Safari"
		if match := regexp.MustCompile(`Version/(\d+)\.(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 2 {
			browser.Version = match[1]
			browser.Full = match[1] + "." + match[2]
		}
	case strings.Contains(rawLower, "msie") || strings.Contains(rawLower, "trident"):
		browser.Name = "IE"
		if match := regexp.MustCompile(`(?:MSIE |rv:)(\d+)\.(\d+)`).FindStringSubmatch(raw); len(match) > 2 {
			browser.Version = match[1]
			browser.Full = match[1] + "." + match[2]
		}
	}

	return browser
}

// parseEngine 解析渲染引擎.
func parseEngine(raw, rawLower string) Engine {
	engine := Engine{}

	switch {
	case strings.Contains(rawLower, "trident"):
		engine.Name = "Trident"
		if match := regexp.MustCompile(`Trident/(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 1 {
			engine.Version = match[1]
		}
	case strings.Contains(rawLower, "gecko") && !strings.Contains(rawLower, "like gecko"):
		engine.Name = "Gecko"
		if match := regexp.MustCompile(`rv:(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 1 {
			engine.Version = match[1]
		}
	case strings.Contains(rawLower, "applewebkit"):
		// Chrome/Edge 使用 Blink，Safari 使用 WebKit
		if strings.Contains(rawLower, "chrome") || strings.Contains(rawLower, "edg") {
			engine.Name = "Blink"
		} else {
			engine.Name = "WebKit"
		}
		if match := regexp.MustCompile(`AppleWebKit/(\d+\.?\d*)`).FindStringSubmatch(raw); len(match) > 1 {
			engine.Version = match[1]
		}
	}

	return engine
}

// parseDevice 解析设备品牌和型号.
func parseDevice(raw, rawLower string) (brand, model string) {
	// iPhone
	if strings.Contains(rawLower, "iphone") {
		return "Apple", "iPhone"
	}
	// iPad
	if strings.Contains(rawLower, "ipad") {
		return "Apple", "iPad"
	}
	// Mac
	if strings.Contains(rawLower, "macintosh") {
		return "Apple", "Mac"
	}

	// Samsung
	if match := regexp.MustCompile(`SM-([A-Z]\d+[A-Z]*)`).FindStringSubmatch(raw); len(match) > 1 {
		return "Samsung", "Galaxy " + match[1]
	}
	if strings.Contains(rawLower, "samsung") {
		return "Samsung", ""
	}

	// Huawei
	if match := regexp.MustCompile(`(?:HUAWEI |HW-)([A-Z0-9-]+)`).FindStringSubmatch(raw); len(match) > 1 {
		return "Huawei", match[1]
	}

	// Xiaomi
	if match := regexp.MustCompile(`(?:MI |Redmi )([A-Z0-9 ]+)`).FindStringSubmatch(raw); len(match) > 1 {
		return "Xiaomi", strings.TrimSpace(match[1])
	}

	// OPPO
	if strings.Contains(rawLower, "oppo") {
		return "OPPO", ""
	}

	// vivo
	if strings.Contains(rawLower, "vivo") {
		return "vivo", ""
	}

	return "", ""
}
