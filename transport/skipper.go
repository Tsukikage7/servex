package transport

import "strings"

// MethodSkipper 方法跳过器，用于中间件跳过指定方法.
type MethodSkipper func(method string) bool

// BuildMethodSkipper 构建方法跳过器.
//
// 支持精确匹配和前缀通配两种模式:
//   - 精确匹配: "/api.user.v1.AuthService/Login"
//   - 前缀通配: "/api.user.v1.AuthService/*" (匹配该服务下所有方法)
func BuildMethodSkipper(methods []string) MethodSkipper {
	exact := make(map[string]bool, len(methods))
	var prefixes []string

	for _, m := range methods {
		if strings.HasSuffix(m, "/*") {
			prefixes = append(prefixes, strings.TrimSuffix(m, "*"))
		} else {
			exact[m] = true
		}
	}

	return func(method string) bool {
		if exact[method] {
			return true
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(method, prefix) {
				return true
			}
		}
		return false
	}
}
