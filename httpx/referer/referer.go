// Package referer 提供 HTTP Referer 头解析功能.
//
// 特性：
//   - 解析来源 URL域名、路径、查询参数
//   - 分类来源类型搜索引擎、社交媒体、直接访问、外部引荐
//   - 提取 UTM 营销追踪参数
//   - HTTP/gRPC 中间件支持
//   - 将解析结果存入 context 供链路使用
//
// 示例：
//
//	handler = referer.HTTPMiddleware()(handler)
//
//	ref := referer.FromContext(ctx)
//	fmt.Println(ref.Type)        // "search"
//	fmt.Println(ref.Source)      // "google"
//	fmt.Println(ref.Domain)      // "www.google.com"
package referer

import (
	"context"
	"net/url"
	"strings"
)

// contextKey context 键类型.
type contextKey string

const (
	refererContextKey contextKey = "referer:referer"
)

// SourceType 来源类型.
type SourceType string

const (
	SourceTypeDirect   SourceType = "direct"   // 直接访问
	SourceTypeSearch   SourceType = "search"   // 搜索引擎
	SourceTypeSocial   SourceType = "social"   // 社交媒体
	SourceTypeReferral SourceType = "referral" // 外部引荐
	SourceTypeInternal SourceType = "internal" // 站内跳转
	SourceTypeEmail    SourceType = "email"    // 邮件营销
	SourceTypePaid     SourceType = "paid"     // 付费广告
	SourceTypeUnknown  SourceType = "unknown"  // 未知
)

// Referer 来源信息.
type Referer struct {
	// Raw 原始 Referer 字符串
	Raw string

	// URL 解析后的 URL
	URL *url.URL

	// Type 来源类型
	Type SourceType

	// Source 来源名称如 "google", "facebook", "twitter"
	Source string

	// Domain 来源域名
	Domain string

	// Path 来源路径
	Path string

	// SearchQuery 搜索关键词如果是搜索引擎来源
	SearchQuery string

	// UTM UTM 追踪参数
	UTM UTMParams
}

// UTMParams UTM 营销追踪参数.
type UTMParams struct {
	Source   string // utm_source
	Medium   string // utm_medium
	Campaign string // utm_campaign
	Term     string // utm_term
	Content  string // utm_content
}

// HasUTM 检查是否有 UTM 参数.
func (u UTMParams) HasUTM() bool {
	return u.Source != "" || u.Medium != "" || u.Campaign != ""
}

// IsEmpty 检查 Referer 是否为空.
func (r *Referer) IsEmpty() bool {
	return r == nil || r.Raw == ""
}

// IsDirect 是否为直接访问.
func (r *Referer) IsDirect() bool {
	return r.Type == SourceTypeDirect
}

// IsSearch 是否来自搜索引擎.
func (r *Referer) IsSearch() bool {
	return r.Type == SourceTypeSearch
}

// IsSocial 是否来自社交媒体.
func (r *Referer) IsSocial() bool {
	return r.Type == SourceTypeSocial
}

// WithReferer 将 Referer 存入 context.
func WithReferer(ctx context.Context, ref *Referer) context.Context {
	return context.WithValue(ctx, refererContextKey, ref)
}

// FromContext 从 context 获取 Referer.
func FromContext(ctx context.Context) (*Referer, bool) {
	ref, ok := ctx.Value(refererContextKey).(*Referer)
	return ref, ok
}

// GetSource 从 context 获取来源名称.
func GetSource(ctx context.Context) string {
	if ref, ok := FromContext(ctx); ok {
		return ref.Source
	}
	return ""
}

// GetSourceType 从 context 获取来源类型.
func GetSourceType(ctx context.Context) SourceType {
	if ref, ok := FromContext(ctx); ok {
		return ref.Type
	}
	return SourceTypeUnknown
}

// GetDomain 从 context 获取来源域名.
func GetDomain(ctx context.Context) string {
	if ref, ok := FromContext(ctx); ok {
		return ref.Domain
	}
	return ""
}

// Parse 解析 Referer 字符串.
func Parse(raw string) *Referer {
	ref := &Referer{
		Raw:  raw,
		Type: SourceTypeDirect,
	}

	if raw == "" {
		return ref
	}

	// 解析 URL
	u, err := url.Parse(raw)
	if err != nil {
		ref.Type = SourceTypeUnknown
		return ref
	}

	ref.URL = u
	ref.Domain = u.Host
	ref.Path = u.Path

	// 提取 UTM 参数
	query := u.Query()
	ref.UTM = UTMParams{
		Source:   query.Get("utm_source"),
		Medium:   query.Get("utm_medium"),
		Campaign: query.Get("utm_campaign"),
		Term:     query.Get("utm_term"),
		Content:  query.Get("utm_content"),
	}

	// 根据 UTM 参数判断类型
	if ref.UTM.HasUTM() {
		switch ref.UTM.Medium {
		case "cpc", "ppc", "paidsearch", "paid":
			ref.Type = SourceTypePaid
			ref.Source = ref.UTM.Source
			return ref
		case "email":
			ref.Type = SourceTypeEmail
			ref.Source = ref.UTM.Source
			return ref
		case "social":
			ref.Type = SourceTypeSocial
			ref.Source = ref.UTM.Source
			return ref
		}
	}

	// 分类来源类型
	ref.Type, ref.Source = classifySource(u.Host, u.Path, query)

	// 提取搜索关键词
	if ref.Type == SourceTypeSearch {
		ref.SearchQuery = extractSearchQuery(ref.Source, query)
	}

	return ref
}

// ParseWithHost 解析 Referer 并判断站内/站外.
func ParseWithHost(raw, currentHost string) *Referer {
	ref := Parse(raw)
	if ref.Domain != "" && isSameDomain(ref.Domain, currentHost) {
		ref.Type = SourceTypeInternal
	}
	return ref
}

// 搜索引擎配置.
var searchEngines = map[string]struct {
	name       string
	queryParam string
}{
	"google":       {"google", "q"},
	"www.google":   {"google", "q"},
	"bing":         {"bing", "q"},
	"www.bing":     {"bing", "q"},
	"baidu":        {"baidu", "wd"},
	"www.baidu":    {"baidu", "wd"},
	"sogou":        {"sogou", "query"},
	"www.sogou":    {"sogou", "query"},
	"so":           {"360", "q"},
	"www.so":       {"360", "q"},
	"yahoo":        {"yahoo", "p"},
	"search.yahoo": {"yahoo", "p"},
	"duckduckgo":   {"duckduckgo", "q"},
	"yandex":       {"yandex", "text"},
	"naver":        {"naver", "query"},
}

// 社交媒体配置.
var socialNetworks = map[string]string{
	"facebook":      "facebook",
	"www.facebook":  "facebook",
	"fb":            "facebook",
	"twitter":       "twitter",
	"x":             "twitter",
	"t.co":          "twitter",
	"instagram":     "instagram",
	"www.instagram": "instagram",
	"linkedin":      "linkedin",
	"www.linkedin":  "linkedin",
	"weibo":         "weibo",
	"www.weibo":     "weibo",
	"weixin":        "wechat",
	"wechat":        "wechat",
	"youtube":       "youtube",
	"www.youtube":   "youtube",
	"tiktok":        "tiktok",
	"www.tiktok":    "tiktok",
	"douyin":        "douyin",
	"pinterest":     "pinterest",
	"www.pinterest": "pinterest",
	"reddit":        "reddit",
	"www.reddit":    "reddit",
	"zhihu":         "zhihu",
	"www.zhihu":     "zhihu",
	"bilibili":      "bilibili",
	"www.bilibili":  "bilibili",
}

// classifySource 分类来源.
func classifySource(host, path string, query url.Values) (SourceType, string) {
	hostLower := strings.ToLower(host)

	// 移除端口号
	if idx := strings.Index(hostLower, ":"); idx != -1 {
		hostLower = hostLower[:idx]
	}

	// 检查搜索引擎
	for prefix, engine := range searchEngines {
		if strings.HasPrefix(hostLower, prefix+".") || hostLower == prefix {
			return SourceTypeSearch, engine.name
		}
	}

	// 检查社交媒体
	for prefix, name := range socialNetworks {
		if strings.HasPrefix(hostLower, prefix+".") || hostLower == prefix || strings.HasSuffix(hostLower, "."+prefix+".com") {
			return SourceTypeSocial, name
		}
	}

	// 默认为外部引荐
	return SourceTypeReferral, extractDomainName(hostLower)
}

// extractSearchQuery 提取搜索关键词.
func extractSearchQuery(source string, query url.Values) string {
	for _, engine := range searchEngines {
		if engine.name == source {
			return query.Get(engine.queryParam)
		}
	}
	return ""
}

// extractDomainName 从主机名提取域名名称.
func extractDomainName(host string) string {
	// 移除 www 前缀
	host = strings.TrimPrefix(host, "www.")

	// 提取主域名
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[0]
	}
	return host
}

// isSameDomain 检查是否为相同域名.
func isSameDomain(domain1, domain2 string) bool {
	d1 := strings.ToLower(strings.TrimPrefix(domain1, "www."))
	d2 := strings.ToLower(strings.TrimPrefix(domain2, "www."))

	// 移除端口
	if idx := strings.Index(d1, ":"); idx != -1 {
		d1 = d1[:idx]
	}
	if idx := strings.Index(d2, ":"); idx != -1 {
		d2 = d2[:idx]
	}

	return d1 == d2
}
