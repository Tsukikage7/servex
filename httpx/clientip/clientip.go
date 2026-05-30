// Package clientip 提供客户端 IP 提取和管理功能.
//
// 特性：
//   - 从 HTTP 请求和 gRPC 调用中提取客户端真实 IP
//   - 支持 X-Forwarded-For、X-Real-IP 等代理头解析
//   - 将 IP 信息存入 context 供整个请求链路使用
//   - 可选的地理位置查询和 IP 访问控制
//
// 示例：
//
//	// HTTP 服务器
//	handler = clientip.HTTPMiddleware()(handler)
//
//	// gRPC 服务器
//	grpc.NewServer(grpc.ChainUnaryInterceptor(
//	    clientip.UnaryServerInterceptor(),
//	))
//
//	// 获取客户端 IP
//	ip := clientip.GetIP(ctx)
package clientip

import (
	"context"
	"net"
	"strings"
)

// contextKey context 键类型.
type contextKey string

const (
	// ipContextKey 客户端 IP 的 context 键.
	ipContextKey contextKey = "clientip:ip"
	// geoContextKey 地理位置信息的 context 键.
	geoContextKey contextKey = "clientip:geo"
)

// IP 客户端 IP 信息.
type IP struct {
	// Address 纯 IP 地址不含端口
	Address string

	// Port 端口可选
	Port string

	// Raw 原始值
	Raw string
}

// String 返回 IP 地址字符串.
func (ip *IP) String() string {
	if ip == nil {
		return ""
	}
	return ip.Address
}

// WithIP 将 IP 信息存入 context.
func WithIP(ctx context.Context, ip *IP) context.Context {
	return context.WithValue(ctx, ipContextKey, ip)
}

// FromContext 从 context 获取 IP 信息.
func FromContext(ctx context.Context) (*IP, bool) {
	ip, ok := ctx.Value(ipContextKey).(*IP)
	return ip, ok
}

// GetIP 从 context 获取 IP 地址字符串.
//
// 便捷方法，如果不存在返回空字符串.
func GetIP(ctx context.Context) string {
	if ip, ok := FromContext(ctx); ok {
		return ip.Address
	}
	return ""
}

// MustFromContext 从 context 获取 IP 信息，不存在时 panic.
func MustFromContext(ctx context.Context) *IP {
	ip, ok := FromContext(ctx)
	if !ok {
		panic("clientip: IP not found in context")
	}
	return ip
}

// ParseIP 解析 IP 地址字符串.
//
// 支持以下格式：
//   - "192.168.1.1"
//   - "192.168.1.1:8080"
//   - "[::1]"
//   - "[::1]:8080"
//   - "::1"
func ParseIP(addr string) *IP {
	if addr == "" {
		return &IP{}
	}

	ip := &IP{Raw: addr}

	// 处理 IPv6 带方括号的格式 [::1]:port
	if strings.HasPrefix(addr, "[") {
		if idx := strings.LastIndex(addr, "]:"); idx != -1 {
			ip.Address = addr[1:idx]
			ip.Port = addr[idx+2:]
			return ip
		}
		// [::1] 无端口
		if strings.HasSuffix(addr, "]") {
			ip.Address = addr[1 : len(addr)-1]
			return ip
		}
	}

	// 尝试分离 host:port
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		ip.Address = host
		ip.Port = port
		return ip
	}

	// 无端口的纯 IP
	ip.Address = addr
	return ip
}

// ParseXForwardedFor 解析 X-Forwarded-For 头.
//
// X-Forwarded-For 格式: "client, proxy1, proxy2"
// 返回第一个 IP最原始的客户端.
func ParseXForwardedFor(xff string) string {
	if xff == "" {
		return ""
	}

	// 取第一个 IP
	if idx := strings.Index(xff, ","); idx != -1 {
		return strings.TrimSpace(xff[:idx])
	}
	return strings.TrimSpace(xff)
}

// ParseXForwardedForWithTrust 解析 X-Forwarded-For 头，跳过可信代理.
//
// 从右向左遍历，找到第一个不在可信代理列表中的 IP.
// 这是更安全的做法，因为攻击者可以伪造 X-Forwarded-For 头的前部分.
func ParseXForwardedForWithTrust(xff string, isTrusted func(ip string) bool) string {
	if xff == "" {
		return ""
	}

	parts := strings.Split(xff, ",")

	// 从右向左遍历
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if !isTrusted(ip) {
			return ip
		}
	}

	// 所有 IP 都是可信代理，返回最左边的
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

// IsPrivateIP 检查 IP 是否为私有地址.
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

// IsValidIP 检查字符串是否为有效的 IP 地址.
func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}
