package clientip

import (
	"context"
	"errors"
	"net"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrIPDenied IP 地址被拒绝.
var ErrIPDenied = errors.New("clientip: IP address denied")

// ACLMode ACL 默认策略模式.
type ACLMode int

const (
	// ACLModeAllowAll 默认允许所有，仅拒绝黑名单中的 IP.
	ACLModeAllowAll ACLMode = iota

	// ACLModeDenyAll 默认拒绝所有，仅允许白名单中的 IP.
	ACLModeDenyAll
)

// ACL IP 访问控制列表.
//
// 支持 CIDR 格式的 IP 范围配置.
type ACL struct {
	allowList []*net.IPNet // 白名单
	denyList  []*net.IPNet // 黑名单
	mode      ACLMode      // 默认策略
}

// ACLOption ACL 配置选项.
type ACLOption func(*ACL)

// NewACL 创建 IP 访问控制列表.
//
// 示例:
//
//	// 默认允许，拒绝特定 IP
//	acl := clientip.NewACL(
//	    clientip.WithDenyList("192.168.1.100", "10.0.0.0/8"),
//	)
//
//	// 默认拒绝，仅允许特定 IP
//	acl := clientip.NewACL(
//	    clientip.WithACLMode(clientip.ACLModeDenyAll),
//	    clientip.WithAllowList("192.168.1.0/24", "10.0.0.1"),
//	)
func NewACL(opts ...ACLOption) *ACL {
	acl := &ACL{
		mode: ACLModeAllowAll,
	}
	for _, opt := range opts {
		opt(acl)
	}
	return acl
}

// WithACLMode 设置 ACL 默认策略模式.
func WithACLMode(mode ACLMode) ACLOption {
	return func(a *ACL) {
		a.mode = mode
	}
}

// WithAllowList 添加白名单.
//
// 支持单个 IP 或 CIDR 格式.
func WithAllowList(cidrs ...string) ACLOption {
	return func(a *ACL) {
		a.allowList = append(a.allowList, parseCIDRs(cidrs)...)
	}
}

// WithDenyList 添加黑名单.
//
// 支持单个 IP 或 CIDR 格式.
func WithDenyList(cidrs ...string) ACLOption {
	return func(a *ACL) {
		a.denyList = append(a.denyList, parseCIDRs(cidrs)...)
	}
}

// parseCIDRs 解析 CIDR 列表.
func parseCIDRs(cidrs []string) []*net.IPNet {
	var result []*net.IPNet
	for _, cidr := range cidrs {
		// 尝试解析为 CIDR
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			result = append(result, ipNet)
			continue
		}

		// 尝试解析为单个 IP
		ip := net.ParseIP(cidr)
		if ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			result = append(result, &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(bits, bits),
			})
		}
	}
	return result
}

// IsAllowed 检查 IP 是否被允许.
func (a *ACL) IsAllowed(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查黑名单优先级最高
	for _, cidr := range a.denyList {
		if cidr.Contains(ip) {
			return false
		}
	}

	// 检查白名单
	for _, cidr := range a.allowList {
		if cidr.Contains(ip) {
			return true
		}
	}

	// 根据默认策略决定
	return a.mode == ACLModeAllowAll
}

// Check 检查 IP 是否被允许，不允许时返回错误.
func (a *ACL) Check(ipStr string) error {
	if !a.IsAllowed(ipStr) {
		return ErrIPDenied
	}
	return nil
}

// AddToAllowList 动态添加 IP 到白名单.
func (a *ACL) AddToAllowList(cidrs ...string) {
	a.allowList = append(a.allowList, parseCIDRs(cidrs)...)
}

// AddToDenyList 动态添加 IP 到黑名单.
func (a *ACL) AddToDenyList(cidrs ...string) {
	a.denyList = append(a.denyList, parseCIDRs(cidrs)...)
}

// ACLHTTPMiddleware 创建 HTTP IP 访问控制中间件.
//
// 需要配合 HTTPMiddleware 使用，从 context 获取客户端 IP.
// 被拒绝的请求返回 403 Forbidden.
//
// 示例:
//
//	acl := clientip.NewACL(
//	    clientip.WithDenyList("192.168.1.100"),
//	)
//
//	handler = clientip.HTTPMiddleware()(handler)          // 先提取 IP
//	handler = clientip.ACLHTTPMiddleware(acl)(handler)    // 再检查 ACL
func ACLHTTPMiddleware(acl *ACL) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetIP(r.Context())
			if ip == "" {
				// 没有 IP 信息，尝试直接从请求获取
				ip = ParseIP(r.RemoteAddr).Address
			}

			if !acl.IsAllowed(ip) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ACLUnaryServerInterceptor 创建一元 gRPC IP 访问控制拦截器.
//
// 需要配合 UnaryServerInterceptor 使用，从 context 获取客户端 IP.
// 被拒绝的请求返回 PermissionDenied 错误.
func ACLUnaryServerInterceptor(acl *ACL) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ip := GetIP(ctx)
		if !acl.IsAllowed(ip) {
			return nil, status.Error(codes.PermissionDenied, "IP address denied")
		}
		return handler(ctx, req)
	}
}

// ACLStreamServerInterceptor 创建流式 gRPC IP 访问控制拦截器.
func ACLStreamServerInterceptor(acl *ACL) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ip := GetIP(ss.Context())
		if !acl.IsAllowed(ip) {
			return status.Error(codes.PermissionDenied, "IP address denied")
		}
		return handler(srv, ss)
	}
}

// CountryACL 基于国家的访问控制.
//
// 需要配合 GeoResolver 使用.
type CountryACL struct {
	allowCountries map[string]bool // 允许的国家代码
	denyCountries  map[string]bool // 拒绝的国家代码
	mode           ACLMode
}

// CountryACLOption 国家 ACL 配置选项.
type CountryACLOption func(*CountryACL)

// NewCountryACL 创建基于国家的访问控制.
//
// 示例:
//
//	// 仅允许中国大陆访问
//	acl := clientip.NewCountryACL(
//	    clientip.WithACLMode(clientip.ACLModeDenyAll),
//	    clientip.WithAllowCountries("CN"),
//	)
//
//	// 禁止特定国家访问
//	acl := clientip.NewCountryACL(
//	    clientip.WithDenyCountries("XX", "YY"),
//	)
func NewCountryACL(opts ...CountryACLOption) *CountryACL {
	acl := &CountryACL{
		allowCountries: make(map[string]bool),
		denyCountries:  make(map[string]bool),
		mode:           ACLModeAllowAll,
	}
	for _, opt := range opts {
		opt(acl)
	}
	return acl
}

// WithCountryACLMode 设置国家 ACL 默认策略模式.
func WithCountryACLMode(mode ACLMode) CountryACLOption {
	return func(a *CountryACL) {
		a.mode = mode
	}
}

// WithAllowCountries 添加允许的国家代码.
//
// 使用 ISO 3166-1 alpha-2 国家代码如 "CN", "US", "JP".
func WithAllowCountries(countries ...string) CountryACLOption {
	return func(a *CountryACL) {
		for _, c := range countries {
			a.allowCountries[c] = true
		}
	}
}

// WithDenyCountries 添加拒绝的国家代码.
func WithDenyCountries(countries ...string) CountryACLOption {
	return func(a *CountryACL) {
		for _, c := range countries {
			a.denyCountries[c] = true
		}
	}
}

// IsAllowed 检查国家是否被允许.
func (a *CountryACL) IsAllowed(country string) bool {
	// 检查黑名单
	if a.denyCountries[country] {
		return false
	}

	// 检查白名单
	if a.allowCountries[country] {
		return true
	}

	return a.mode == ACLModeAllowAll
}

// Check 从 context 检查国家是否被允许.
//
// 需要配合 GeoResolver 使用.
func (a *CountryACL) Check(ctx context.Context) error {
	geo, ok := GeoInfoFromContext(ctx)
	if !ok {
		// 没有地理位置信息，根据默认策略决定
		if a.mode == ACLModeDenyAll {
			return ErrIPDenied
		}
		return nil
	}

	if !a.IsAllowed(geo.Country) {
		return ErrIPDenied
	}
	return nil
}

// CountryACLHTTPMiddleware 创建基于国家的 HTTP 访问控制中间件.
//
// 需要配合 HTTPMiddleware + GeoResolver 使用.
func CountryACLHTTPMiddleware(acl *CountryACL) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := acl.Check(r.Context()); err != nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CountryACLUnaryServerInterceptor 创建基于国家的一元 gRPC 访问控制拦截器.
func CountryACLUnaryServerInterceptor(acl *CountryACL) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := acl.Check(ctx); err != nil {
			return nil, status.Error(codes.PermissionDenied, "access denied by country")
		}
		return handler(ctx, req)
	}
}
