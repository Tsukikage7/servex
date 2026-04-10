package httpserver

import (
	"context"
	"io"
	"net/http"

	"github.com/Tsukikage7/servex/encoding"
	// 注册默认编解码器
	_ "github.com/Tsukikage7/servex/encoding/json"
	_ "github.com/Tsukikage7/servex/encoding/proto"
	_ "github.com/Tsukikage7/servex/encoding/xml"
)

// EncodeCodecResponse 基于 Accept 头自动选择编解码器的响应编码函数.
// 可直接作为 EncodeResponseFunc 使用.
func EncodeCodecResponse(ctx context.Context, w http.ResponseWriter, resp any) error {
	r := requestFromContext(ctx)
	codec := encoding.CodecForRequest(r, "Accept")
	if codec == nil {
		// 不应发生，CodecForRequest 有 JSON 回退
		return EncodeJSONResponse(ctx, w, resp)
	}

	data, err := codec.Marshal(resp)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", contentTypeForCodec(codec.Name()))
	_, err = w.Write(data)
	return err
}

// DefaultMaxBodySize 默认请求体最大大小（10MB），可通过 DecodeCodecRequestWithLimit 自定义.
const DefaultMaxBodySize = 10 << 20

// DecodeCodecRequest 基于 Content-Type 头自动选择编解码器的请求解码函数.
// 泛型辅助，返回 DecodeRequestFunc.
func DecodeCodecRequest[T any]() DecodeRequestFunc {
	return DecodeCodecRequestWithLimit[T](DefaultMaxBodySize)
}

// DecodeCodecRequestWithLimit 基于 Content-Type 头自动选择编解码器的请求解码函数（可配置请求体大小限制）.
func DecodeCodecRequestWithLimit[T any](maxBodySize int64) DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (any, error) {
		codec := encoding.CodecForRequest(r, "Content-Type")
		if codec == nil {
			return nil, encoding.ErrCodecNotFound
		}

		// 限制请求体大小，防止超大请求导致 OOM
		defer r.Body.Close()
		data, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			return nil, err
		}

		var req T
		if err := codec.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		return req, nil
	}
}

// contentTypeForCodec 根据编解码器名称返回 Content-Type.
func contentTypeForCodec(name string) string {
	switch name {
	case "json":
		return "application/json; charset=utf-8"
	case "xml":
		return "application/xml; charset=utf-8"
	case "proto":
		return "application/x-protobuf"
	default:
		return "application/octet-stream"
	}
}

// requestContextKey 用于在 context 中存储 *http.Request.
type requestContextKey struct{}

// WithRequest 将 *http.Request 存入 context.
// 用于 EncodeCodecResponse 从 context 中获取请求头.
func WithRequest() RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, requestContextKey{}, r)
	}
}

// requestFromContext 从 context 中获取 *http.Request.
func requestFromContext(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestContextKey{}).(*http.Request)
	if r == nil {
		// 如果 context 中没有 request，构建空请求
		r, _ = http.NewRequest(http.MethodGet, "/", nil)
	}
	return r
}
