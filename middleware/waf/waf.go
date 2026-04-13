// Package waf 提供 Web Application Firewall 中间件.
// 基于 OWASP 规则引擎，支持 SQL 注入、XSS、路径遍历和命令注入检测.
package waf

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Rule 是 WAF 规则接口.
// Match 检查请求是否匹配规则，返回是否匹配及匹配原因.
type Rule interface {
	Match(r *http.Request) (bool, string)
}

// Options 配置选项.
type Options struct {
	// Rules WAF 规则列表.
	Rules []Rule

	// BlockHandler 自定义拦截处理函数.
	// 如果为 nil，使用默认处理（返回 403）.
	BlockHandler func(w http.ResponseWriter, r *http.Request, reason string)

	// Logger 日志记录器.
	Logger logger.Logger
}

// Option 是配置函数.
type Option func(*Options)

// WithRules 添加 WAF 规则.
func WithRules(rules ...Rule) Option {
	return func(o *Options) {
		o.Rules = append(o.Rules, rules...)
	}
}

// WithBlockHandler 设置自定义拦截处理函数.
func WithBlockHandler(fn func(w http.ResponseWriter, r *http.Request, reason string)) Option {
	return func(o *Options) {
		o.BlockHandler = fn
	}
}

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// defaultOptions 返回默认配置.
func defaultOptions() *Options {
	return &Options{}
}

// applyOptions 应用配置选项.
func applyOptions(opts []Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// HTTPMiddleware 返回 HTTP WAF 中间件.
// 当请求匹配任意规则时，记录日志并返回 403 Forbidden.
func HTTPMiddleware(opts ...Option) func(http.Handler) http.Handler {
	o := applyOptions(opts)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, rule := range o.Rules {
				matched, reason := rule.Match(r)
				if matched {
					// 记录日志
					if o.Logger != nil {
						o.Logger.Warn("WAF 拦截请求",
							logger.String("method", r.Method),
							logger.String("path", r.URL.Path),
							logger.String("remote_addr", r.RemoteAddr),
							logger.String("reason", reason),
						)
					}

					// 调用自定义拦截处理或默认处理
					if o.BlockHandler != nil {
						o.BlockHandler(w, r, reason)
						return
					}

					http.Error(w, reason, http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// regexRule 基于正则表达式的规则实现.
type regexRule struct {
	name     string
	patterns []*regexp.Regexp
}

// Match 检查请求是否匹配正则规则.
// 扫描 URL、查询参数和请求体（Content-Type 为 form 时）.
func (r *regexRule) Match(req *http.Request) (bool, string) {
	// 检查 URL 路径
	if r.matchString(req.URL.Path) {
		return true, r.name + ": URL 路径包含恶意内容"
	}

	// 检查查询参数
	for key, values := range req.URL.Query() {
		if r.matchString(key) {
			return true, r.name + ": 查询参数名包含恶意内容"
		}
		for _, v := range values {
			if r.matchString(v) {
				return true, r.name + ": 查询参数值包含恶意内容"
			}
		}
	}

	// 检查请求头中常见的注入点
	for _, header := range []string{"Referer", "User-Agent", "Cookie"} {
		if val := req.Header.Get(header); val != "" {
			if r.matchString(val) {
				return true, r.name + ": 请求头 " + header + " 包含恶意内容"
			}
		}
	}

	return false, ""
}

// matchString 检查字符串是否匹配任意模式.
func (r *regexRule) matchString(s string) bool {
	lower := strings.ToLower(s)
	for _, p := range r.patterns {
		if p.MatchString(lower) {
			return true
		}
	}
	return false
}

// SQLInjectionRule 返回 SQL 注入检测规则.
// 检测常见的 SQL 注入模式，包括 UNION SELECT、OR 1=1、注释注入等.
func SQLInjectionRule() Rule {
	return &regexRule{
		name: "SQL 注入",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\bunion\b.*\bselect\b)`),
			regexp.MustCompile(`(?i)(\bselect\b.*\bfrom\b)`),
			regexp.MustCompile(`(?i)(\binsert\b.*\binto\b)`),
			regexp.MustCompile(`(?i)(\bdelete\b.*\bfrom\b)`),
			regexp.MustCompile(`(?i)(\bdrop\b.*\b(table|database)\b)`),
			regexp.MustCompile(`(?i)(\bor\b\s+\d+\s*=\s*\d+)`),
			regexp.MustCompile(`(?i)(\band\b\s+\d+\s*=\s*\d+)`),
			regexp.MustCompile(`('|\");\s*(drop|alter|create|delete|insert|update)`),
			regexp.MustCompile(`(--|#|/\*)\s*$`),
			regexp.MustCompile(`(?i)\bexec\b.*\bxp_`),
		},
	}
}

// XSSRule 返回 XSS 检测规则.
// 检测常见的跨站脚本攻击模式，包括 script 标签、事件处理器、javascript: 协议等.
func XSSRule() Rule {
	return &regexRule{
		name: "XSS",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)<script[\s>]`),
			regexp.MustCompile(`(?i)</script>`),
			regexp.MustCompile(`(?i)\bon\w+\s*=`),
			regexp.MustCompile(`(?i)javascript\s*:`),
			regexp.MustCompile(`(?i)vbscript\s*:`),
			regexp.MustCompile(`(?i)<iframe[\s>]`),
			regexp.MustCompile(`(?i)<object[\s>]`),
			regexp.MustCompile(`(?i)<embed[\s>]`),
			regexp.MustCompile(`(?i)<img[^>]+\bon\w+`),
		},
	}
}

// PathTraversalRule 返回路径遍历检测规则.
// 检测目录遍历攻击，如 ../ 和编码变体.
func PathTraversalRule() Rule {
	return &regexRule{
		name: "路径遍历",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`\.\./`),
			regexp.MustCompile(`\.\.\\`),
			regexp.MustCompile(`%2e%2e[/\\]`),
			regexp.MustCompile(`%252e%252e[/\\]`),
			regexp.MustCompile(`\.\./\.\./`),
			regexp.MustCompile(`/etc/passwd`),
			regexp.MustCompile(`/etc/shadow`),
			regexp.MustCompile(`(?i)c:\\windows`),
		},
	}
}

// CommandInjectionRule 返回命令注入检测规则.
// 检测 OS 命令注入模式，如管道符、反引号、$() 等.
func CommandInjectionRule() Rule {
	return &regexRule{
		name: "命令注入",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`[;&|]{2,}`),
			regexp.MustCompile("`.+`"),
			regexp.MustCompile(`\$\(.+\)`),
			regexp.MustCompile(`(?i)\b(cat|ls|whoami|id|uname|curl|wget|nc|bash|sh|cmd|powershell)\b`),
			regexp.MustCompile(`[|;]\s*(cat|ls|whoami|id|uname)\b`),
		},
	}
}
