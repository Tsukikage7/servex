// Package bodylimit 提供 HTTP 请求体大小限制中间件.
// 防止客户端发送过大的请求体导致服务器资源耗尽.
package bodylimit

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// ErrBodyTooLarge 请求体超出大小限制.
var ErrBodyTooLarge = errors.New("bodylimit: request body too large")

// HTTPMiddleware 限制请求体大小.
// limit 为最大字节数，如 1<<20 (1MB).
// 超出限制时返回 413 Request Entity Too Large.
func HTTPMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content-Length 快速检查
			if r.ContentLength > limit {
				http.Error(w, ErrBodyTooLarge.Error(), http.StatusRequestEntityTooLarge)
				return
			}

			// 用 MaxBytesReader 包装 body，防止实际传输超限
			r.Body = http.MaxBytesReader(w, r.Body, limit)

			next.ServeHTTP(w, r)
		})
	}
}

// 匹配人类可读的大小字符串，如 "1MB", "512KB", "10.5GB".
var sizeRegex = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)\s*$`)

// ParseLimit 解析人类可读的大小字符串如 "1MB", "512KB", "10GB".
// 支持的单位: B, KB, MB, GB, TB不区分大小写.
func ParseLimit(s string) (int64, error) {
	matches := sizeRegex.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("bodylimit: invalid size format: %q", s)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("bodylimit: invalid number: %q", matches[1])
	}

	unit := strings.ToUpper(matches[2])
	var multiplier float64
	switch unit {
	case "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(value * multiplier), nil
}
