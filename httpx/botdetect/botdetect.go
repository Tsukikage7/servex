// Package botdetect 提供机器人/爬虫检测功能.
//
// 特性：
//   - 基于 User-Agent 的模式检测
//   - 已知机器人数据库搜索引擎、社交媒体等
//   - 机器人分类好机器人、坏机器人、未知
//   - 置信度评分
//   - HTTP/gRPC 中间件支持
//
// 示例：
//
//	handler = botdetect.HTTPMiddleware()(handler)
//
//	result := botdetect.FromContext(ctx)
//	fmt.Println(result.IsBot)         // true
//	fmt.Println(result.Category)      // "search"
//	fmt.Println(result.Name)          // "Googlebot"
//	fmt.Println(result.Confidence)    // 0.95
package botdetect

import (
	"context"
	"regexp"
	"strings"
)

// contextKey context 键类型.
type contextKey string

const (
	botContextKey contextKey = "botdetect:result"
)

// Category 机器人分类.
type Category string

const (
	CategoryHuman    Category = "human"    // 人类用户
	CategorySearch   Category = "search"   // 搜索引擎
	CategorySocial   Category = "social"   // 社交媒体
	CategoryMonitor  Category = "monitor"  // 监控/健康检查
	CategoryFeed     Category = "feed"     // RSS/Feed 读取器
	CategoryScraper  Category = "scraper"  // 爬虫/抓取器
	CategorySpam     Category = "spam"     // 垃圾机器人
	CategorySecurity Category = "security" // 安全扫描
	CategoryTool     Category = "tool"     // 开发/测试工具
	CategoryUnknown  Category = "unknown"  // 未知
)

// Intent 机器人意图.
type Intent string

const (
	IntentGood    Intent = "good"    // 良性机器人如搜索引擎
	IntentBad     Intent = "bad"     // 恶意机器人
	IntentNeutral Intent = "neutral" // 中性未知意图
)

// Result 检测结果.
type Result struct {
	// IsBot 是否为机器人
	IsBot bool

	// Category 机器人分类
	Category Category

	// Intent 机器人意图
	Intent Intent

	// Name 机器人名称如 "Googlebot"
	Name string

	// Company 所属公司/组织
	Company string

	// URL 机器人信息页面
	URL string

	// Confidence 置信度 (0.0 - 1.0)
	Confidence float64

	// Reasons 检测原因
	Reasons []string

	// Raw 原始 User-Agent
	Raw string
}

// IsGoodBot 是否为良性机器人.
func (r *Result) IsGoodBot() bool {
	return r.IsBot && r.Intent == IntentGood
}

// IsBadBot 是否为恶意机器人.
func (r *Result) IsBadBot() bool {
	return r.IsBot && r.Intent == IntentBad
}

// WithResult 将检测结果存入 context.
func WithResult(ctx context.Context, result *Result) context.Context {
	return context.WithValue(ctx, botContextKey, result)
}

// FromContext 从 context 获取检测结果.
func FromContext(ctx context.Context) (*Result, bool) {
	result, ok := ctx.Value(botContextKey).(*Result)
	return result, ok
}

// IsBot 从 context 检查是否为机器人.
func IsBot(ctx context.Context) bool {
	if result, ok := FromContext(ctx); ok {
		return result.IsBot
	}
	return false
}

// GetBotName 从 context 获取机器人名称.
func GetBotName(ctx context.Context) string {
	if result, ok := FromContext(ctx); ok {
		return result.Name
	}
	return ""
}

// GetCategory 从 context 获取机器人分类.
func GetCategory(ctx context.Context) Category {
	if result, ok := FromContext(ctx); ok {
		return result.Category
	}
	return CategoryUnknown
}

// Detector 机器人检测器.
type Detector struct {
	opts *options
}

// New 创建机器人检测器.
func New(opts ...Option) *Detector {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Detector{opts: o}
}

// Detect 检测 User-Agent.
func (d *Detector) Detect(userAgent string) *Result {
	result := &Result{
		Raw:        userAgent,
		Category:   CategoryHuman,
		Intent:     IntentNeutral,
		Confidence: 0.0,
		Reasons:    make([]string, 0),
	}

	if userAgent == "" {
		result.IsBot = true
		result.Category = CategoryUnknown
		result.Confidence = 0.7
		result.Reasons = append(result.Reasons, "empty user-agent")
		return result
	}

	uaLower := strings.ToLower(userAgent)

	// 检查已知机器人
	for _, bot := range knownBots {
		if bot.match(uaLower) {
			result.IsBot = true
			result.Name = bot.name
			result.Company = bot.company
			result.Category = bot.category
			result.Intent = bot.intent
			result.URL = bot.url
			result.Confidence = 0.95
			result.Reasons = append(result.Reasons, "known bot: "+bot.name)
			return result
		}
	}

	// 检查通用模式
	confidence := 0.0
	reasons := make([]string, 0)

	// Bot/Spider/Crawler 关键词
	for _, pattern := range botPatterns {
		if strings.Contains(uaLower, pattern) {
			confidence += 0.3
			reasons = append(reasons, "bot pattern: "+pattern)
		}
	}

	// HTTP 客户端库
	for _, pattern := range httpClientPatterns {
		if strings.Contains(uaLower, pattern) {
			confidence += 0.4
			reasons = append(reasons, "http client: "+pattern)
		}
	}

	// 编程语言标识
	for _, pattern := range programmingLangPatterns {
		if strings.Contains(uaLower, pattern) {
			confidence += 0.3
			reasons = append(reasons, "programming language: "+pattern)
		}
	}

	// 可疑特征
	for _, pattern := range suspiciousPatterns {
		if pattern.regex.MatchString(uaLower) {
			confidence += pattern.weight
			reasons = append(reasons, "suspicious: "+pattern.name)
		}
	}

	// 缺少常见浏览器标识
	if !hasBrowserSignature(uaLower) {
		confidence += 0.2
		reasons = append(reasons, "missing browser signature")
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	if confidence >= d.opts.threshold {
		result.IsBot = true
		result.Category = CategoryScraper
		result.Confidence = confidence
		result.Reasons = reasons
	}

	return result
}

// 已知机器人数据库.
type botInfo struct {
	name     string
	company  string
	category Category
	intent   Intent
	url      string
	patterns []string
}

func (b *botInfo) match(uaLower string) bool {
	for _, p := range b.patterns {
		if strings.Contains(uaLower, p) {
			return true
		}
	}
	return false
}

var knownBots = []botInfo{
	// 搜索引擎
	{name: "Googlebot", company: "Google", category: CategorySearch, intent: IntentGood, url: "https://developers.google.com/search/docs/crawling-indexing/googlebot", patterns: []string{"googlebot"}},
	{name: "Bingbot", company: "Microsoft", category: CategorySearch, intent: IntentGood, url: "https://www.bing.com/webmaster/help/which-crawlers-does-bing-use", patterns: []string{"bingbot"}},
	{name: "Baiduspider", company: "Baidu", category: CategorySearch, intent: IntentGood, url: "https://www.baidu.com/search/spider.html", patterns: []string{"baiduspider"}},
	{name: "YandexBot", company: "Yandex", category: CategorySearch, intent: IntentGood, url: "https://yandex.com/support/webmaster/robot-workings/check-yandex-robots.html", patterns: []string{"yandexbot", "yandex.com/bots"}},
	{name: "DuckDuckBot", company: "DuckDuckGo", category: CategorySearch, intent: IntentGood, url: "https://duckduckgo.com/duckduckbot", patterns: []string{"duckduckbot"}},
	{name: "Sogou Spider", company: "Sogou", category: CategorySearch, intent: IntentGood, url: "https://www.sogou.com/docs/help/webmasters.htm", patterns: []string{"sogou"}},
	{name: "360Spider", company: "360", category: CategorySearch, intent: IntentGood, patterns: []string{"360spider"}},

	// 社交媒体
	{name: "Facebot", company: "Facebook", category: CategorySocial, intent: IntentGood, url: "https://developers.facebook.com/docs/sharing/bot", patterns: []string{"facebookexternalhit", "facebot"}},
	{name: "Twitterbot", company: "Twitter/X", category: CategorySocial, intent: IntentGood, url: "https://developer.twitter.com/en/docs/twitter-for-websites/cards/guides/getting-started", patterns: []string{"twitterbot"}},
	{name: "LinkedInBot", company: "LinkedIn", category: CategorySocial, intent: IntentGood, patterns: []string{"linkedinbot"}},
	{name: "Slackbot", company: "Slack", category: CategorySocial, intent: IntentGood, patterns: []string{"slackbot"}},
	{name: "WhatsApp", company: "WhatsApp", category: CategorySocial, intent: IntentGood, patterns: []string{"whatsapp"}},
	{name: "TelegramBot", company: "Telegram", category: CategorySocial, intent: IntentGood, patterns: []string{"telegrambot"}},
	{name: "Discordbot", company: "Discord", category: CategorySocial, intent: IntentGood, patterns: []string{"discordbot"}},

	// 监控/健康检查
	{name: "UptimeRobot", company: "UptimeRobot", category: CategoryMonitor, intent: IntentGood, patterns: []string{"uptimerobot"}},
	{name: "Pingdom", company: "Pingdom", category: CategoryMonitor, intent: IntentGood, patterns: []string{"pingdom"}},
	{name: "StatusCake", company: "StatusCake", category: CategoryMonitor, intent: IntentGood, patterns: []string{"statuscake"}},
	{name: "Site24x7", company: "Site24x7", category: CategoryMonitor, intent: IntentGood, patterns: []string{"site24x7"}},

	// Feed 读取器
	{name: "Feedly", company: "Feedly", category: CategoryFeed, intent: IntentGood, patterns: []string{"feedly"}},
	{name: "Feedbin", company: "Feedbin", category: CategoryFeed, intent: IntentGood, patterns: []string{"feedbin"}},

	// 安全扫描
	{name: "Nessus", company: "Tenable", category: CategorySecurity, intent: IntentNeutral, patterns: []string{"nessus"}},
	{name: "Nikto", company: "Open Source", category: CategorySecurity, intent: IntentNeutral, patterns: []string{"nikto"}},
	{name: "Qualys", company: "Qualys", category: CategorySecurity, intent: IntentNeutral, patterns: []string{"qualys"}},

	// 开发工具
	{name: "curl", company: "Open Source", category: CategoryTool, intent: IntentNeutral, patterns: []string{"curl/"}},
	{name: "wget", company: "Open Source", category: CategoryTool, intent: IntentNeutral, patterns: []string{"wget/"}},
	{name: "HTTPie", company: "Open Source", category: CategoryTool, intent: IntentNeutral, patterns: []string{"httpie"}},
	{name: "Postman", company: "Postman", category: CategoryTool, intent: IntentNeutral, patterns: []string{"postman"}},
	{name: "Insomnia", company: "Kong", category: CategoryTool, intent: IntentNeutral, patterns: []string{"insomnia"}},
}

// 通用机器人模式
var botPatterns = []string{
	"bot", "spider", "crawler", "scraper", "fetch",
	"archiver", "indexer", "checker", "validator",
}

// HTTP 客户端库模式
var httpClientPatterns = []string{
	"httpclient", "http_client", "http-client",
	"axios", "node-fetch", "got/", "request/",
	"okhttp", "apache-httpclient", "jersey",
}

// 编程语言模式
var programmingLangPatterns = []string{
	"python", "python-requests", "python-urllib",
	"java/", "java ", "go-http-client", "go ",
	"ruby", "perl", "php/", "php ",
	"node.js", "nodejs",
}

// 可疑模式
type suspiciousPattern struct {
	name   string
	regex  *regexp.Regexp
	weight float64
}

var suspiciousPatterns = []suspiciousPattern{
	{name: "empty_or_short", regex: regexp.MustCompile(`^.{0,10}$`), weight: 0.3},
	{name: "only_mozilla", regex: regexp.MustCompile(`^mozilla/\d+\.\d+$`), weight: 0.4},
	{name: "missing_version", regex: regexp.MustCompile(`^[a-z]+$`), weight: 0.3},
}

// hasBrowserSignature 检查是否有浏览器标识
func hasBrowserSignature(uaLower string) bool {
	browserSignatures := []string{
		"chrome/", "firefox/", "safari/", "edge/", "opera/",
		"msie", "trident/", "webkit",
	}
	for _, sig := range browserSignatures {
		if strings.Contains(uaLower, sig) {
			return true
		}
	}
	return false
}
