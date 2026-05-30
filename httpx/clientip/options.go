package clientip

import "net"

// Option 配置选项.
type Option func(*options)

// options 内部配置.
type options struct {
	// 可信代理列表CIDR 格式
	trustedProxies []*net.IPNet

	// 是否信任所有代理默认 true
	trustAllProxies bool

	// Header 名称
	forwardedHeader string
	realIPHeader    string

	// 地理位置解析器
	geoResolver GeoResolver
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		trustAllProxies: true, // 默认信任所有代理
		forwardedHeader: "X-Forwarded-For",
		realIPHeader:    "X-Real-IP",
	}
}

// WithTrustedProxies 设置可信代理列表.
//
// 只有来自可信代理的转发头才会被信任.
// 支持单个 IP 或 CIDR 格式.
//
// 示例:
//
//	WithTrustedProxies("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")
func WithTrustedProxies(cidrs ...string) Option {
	return func(o *options) {
		o.trustAllProxies = false
		for _, cidr := range cidrs {
			// 尝试解析为 CIDR
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				o.trustedProxies = append(o.trustedProxies, ipNet)
				continue
			}

			// 尝试解析为单个 IP
			ip := net.ParseIP(cidr)
			if ip != nil {
				// 转换为 /32 或 /128 CIDR
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				o.trustedProxies = append(o.trustedProxies, &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(bits, bits),
				})
			}
		}
	}
}

// WithTrustAllProxies 信任所有代理.
//
// 这是默认行为，适合内网环境.
// 生产环境建议使用 WithTrustedProxies 明确指定可信代理.
func WithTrustAllProxies() Option {
	return func(o *options) {
		o.trustAllProxies = true
		o.trustedProxies = nil
	}
}

// WithTrustPrivateProxies 信任所有私有网络代理.
//
// 包括: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8
func WithTrustPrivateProxies() Option {
	return func(o *options) {
		o.trustAllProxies = false
		privateCIDRs := []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
			"::1/128",
			"fc00::/7",
			"fe80::/10",
		}
		for _, cidr := range privateCIDRs {
			_, ipNet, _ := net.ParseCIDR(cidr)
			if ipNet != nil {
				o.trustedProxies = append(o.trustedProxies, ipNet)
			}
		}
	}
}

// WithForwardedHeader 设置转发 header 名称.
//
// 默认: "X-Forwarded-For"
func WithForwardedHeader(name string) Option {
	return func(o *options) {
		o.forwardedHeader = name
	}
}

// WithRealIPHeader 设置真实 IP header 名称.
//
// 默认: "X-Real-IP"
func WithRealIPHeader(name string) Option {
	return func(o *options) {
		o.realIPHeader = name
	}
}

// WithGeoResolver 设置地理位置解析器.
//
// 启用后，中间件会自动解析 IP 的地理位置信息.
func WithGeoResolver(resolver GeoResolver) Option {
	return func(o *options) {
		o.geoResolver = resolver
	}
}

// isTrustedProxy 检查 IP 是否为可信代理.
func (o *options) isTrustedProxy(ipStr string) bool {
	if o.trustAllProxies {
		return true
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range o.trustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}
