package jwt

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

// Whitelist 白名单配置.
type Whitelist struct {
	// HTTPPaths HTTP 路径白名单支持前缀匹配.
	HTTPPaths []string

	// GRPCMethods gRPC 方法白名单支持前缀匹配.
	GRPCMethods []string

	// InternalServiceHeader 内部服务标识 Header.
	//
	// 默认: "x-internal-service".
	InternalServiceHeader string

	// InternalServiceSecret 内部服务共享密钥.
	//
	// 如果为空，则禁用内部服务 Header 旁路.
	// 如果设置，则 Header 的值必须与此密钥匹配才能跳过认证.
	InternalServiceSecret string
}

// NewWhitelist 创建白名单.
func NewWhitelist() *Whitelist {
	return &Whitelist{
		InternalServiceHeader: "x-internal-service",
	}
}

// AddHTTPPaths 添加 HTTP 路径白名单.
func (w *Whitelist) AddHTTPPaths(paths ...string) *Whitelist {
	w.HTTPPaths = append(w.HTTPPaths, paths...)
	return w
}

// AddGRPCMethods 添加 gRPC 方法白名单.
func (w *Whitelist) AddGRPCMethods(methods ...string) *Whitelist {
	w.GRPCMethods = append(w.GRPCMethods, methods...)
	return w
}

// SetInternalServiceHeader 设置内部服务标识 Header.
func (w *Whitelist) SetInternalServiceHeader(header string) *Whitelist {
	w.InternalServiceHeader = header
	return w
}

// SetInternalServiceSecret 设置内部服务共享密钥.
//
// 如果为空，则禁用内部服务 Header 旁路.
func (w *Whitelist) SetInternalServiceSecret(secret string) *Whitelist {
	w.InternalServiceSecret = secret
	return w
}

// IsWhitelisted 检查请求是否在白名单中.
func (w *Whitelist) IsWhitelisted(ctx context.Context, req any) bool {
	if w == nil {
		return false
	}

	// 检查内部服务调用
	if w.isInternalService(req) {
		return true
	}

	// 检查 HTTP 请求
	if httpReq, ok := req.(*http.Request); ok {
		return w.isHTTPPathWhitelisted(httpReq.URL.Path)
	}

	return false
}

// isInternalService 检查是否为内部服务调用.
func (w *Whitelist) isInternalService(req any) bool {
	if w.InternalServiceHeader == "" {
		return false
	}

	// 如果未设置共享密钥，则禁用内部服务旁路
	if w.InternalServiceSecret == "" {
		return false
	}

	httpReq, ok := req.(*http.Request)
	if !ok {
		return false
	}

	service := httpReq.Header.Get(w.InternalServiceHeader)
	if service == "" {
		return false
	}

	// 使用常量时间比较防止时序攻击
	return subtle.ConstantTimeCompare([]byte(service), []byte(w.InternalServiceSecret)) == 1
}

// isHTTPPathWhitelisted 检查 HTTP 路径是否在白名单.
// 使用精确匹配或 "/" 分隔符边界匹配，防止 "/api/public" 意外匹配 "/api/publicData".
func (w *Whitelist) isHTTPPathWhitelisted(path string) bool {
	for _, whitelistPath := range w.HTTPPaths {
		if path == whitelistPath {
			return true
		}
		// 前缀匹配要求下一个字符为 "/" 或路径已到尽头
		if strings.HasPrefix(path, whitelistPath) &&
			(strings.HasSuffix(whitelistPath, "/") || len(path) > len(whitelistPath) && path[len(whitelistPath)] == '/') {
			return true
		}
	}
	return false
}

// isGRPCMethodWhitelisted 检查 gRPC 方法是否在白名单.
// 使用精确匹配或 "/" 分隔符边界匹配.
func (w *Whitelist) isGRPCMethodWhitelisted(method string) bool {
	for _, whitelistMethod := range w.GRPCMethods {
		if method == whitelistMethod {
			return true
		}
		if strings.HasPrefix(method, whitelistMethod) &&
			(strings.HasSuffix(whitelistMethod, "/") || len(method) > len(whitelistMethod) && method[len(whitelistMethod)] == '/') {
			return true
		}
	}
	return false
}
