// Package encoding 提供编解码器接口和全局注册表，支持 HTTP 内容协商.
package encoding

import (
	"errors"
	"net/http"
	"strings"
	"sync"
)

// ErrCodecNotFound 未找到匹配的编解码器.
var ErrCodecNotFound = errors.New("encoding: 未找到匹配的编解码器")

// Codec 编解码器接口.
type Codec interface {
	// Marshal 将值编码为字节.
	Marshal(v any) ([]byte, error)
	// Unmarshal 将字节解码到值.
	Unmarshal(data []byte, v any) error
	// Name 返回编解码器名称（内容子类型），如 "json", "xml", "proto".
	Name() string
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Codec)
)

// RegisterCodec 注册编解码器到全局注册表.
// 重复注册同名编解码器会覆盖.
func RegisterCodec(codec Codec) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[codec.Name()] = codec
}

// GetCodec 按名称获取编解码器.
// 未找到时返回 nil.
func GetCodec(name string) Codec {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// 常见 MIME 类型到编解码器子类型的缓存，避免每次解析.
var commonMIME = map[string]string{
	"application/json":                "json",
	"application/json; charset=utf-8": "json",
	"application/xml":                 "xml",
	"application/xml; charset=utf-8":  "xml",
	"text/xml":                        "xml",
	"text/xml; charset=utf-8":         "xml",
	"application/x-protobuf":          "proto",
	"application/protobuf":            "proto",
	"application/grpc+proto":          "proto",
}

// CodecForRequest 根据 HTTP 请求头选择编解码器.
// headerName 通常为 "Content-Type"（解码）或 "Accept"（编码）.
// 解析 MIME 类型的 subtype 部分（如 "application/xml" -> "xml"）.
// 未匹配时回退到 JSON；若 JSON 编解码器也未注册则返回 nil.
func CodecForRequest(r *http.Request, headerName string) Codec {
	header := r.Header.Get(headerName)
	// 快速路径：常见 MIME 类型直接查缓存
	if name, ok := commonMIME[header]; ok {
		if c := GetCodec(name); c != nil {
			return c
		}
	}
	name := subtypeFromHeader(header)
	if c := GetCodec(name); c != nil {
		return c
	}
	// 回退到 JSON；若未注册则遍历已注册编解码器返回第一个，都没有则返回 nil
	if c := GetCodec("json"); c != nil {
		return c
	}
	return nil
}

// subtypeFromHeader 从 MIME 类型中提取子类型.
// "application/json; charset=utf-8" -> "json"
// "application/x-protobuf" -> "proto"（移除 x- 前缀后映射）
// "text/xml" -> "xml"
func subtypeFromHeader(contentType string) string {
	if contentType == "" {
		return ""
	}
	// 去除参数部分（charset 等）
	if idx := strings.IndexByte(contentType, ';'); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)

	// 取 subtype: "application/json" -> "json"
	if slash := strings.IndexByte(contentType, '/'); slash != -1 {
		sub := contentType[slash+1:]
		// 处理 "x-protobuf" -> "proto" 等前缀
		sub = strings.TrimPrefix(sub, "x-")
		// 处理 "vnd.api+json" -> "json" 等后缀类型
		if plus := strings.LastIndexByte(sub, '+'); plus != -1 {
			sub = sub[plus+1:]
		}
		// 特殊映射
		switch sub {
		case "protobuf":
			return "proto"
		default:
			return sub
		}
	}
	return contentType
}
