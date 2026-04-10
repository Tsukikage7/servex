package pbjson

import (
	"context"
	"io"
	"net/http"

	"google.golang.org/protobuf/proto"
)

// EncodeResponse 将 proto 消息编码为 HTTP JSON 响应.
// 零值字段会被输出.
func EncodeResponse(_ context.Context, w http.ResponseWriter, response any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	msg, ok := response.(proto.Message)
	if !ok {
		return ErrNotProtoMessage
	}

	data, err := Marshal(msg)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// DecodeRequest 从 HTTP 请求体解码 proto 消息.
func DecodeRequest[T proto.Message](r *http.Request, msg T) error {
	defer r.Body.Close()
	// 限制请求体大小为 10MB，防止 OOM
	const maxBodySize = 10 << 20
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		return err
	}

	return Unmarshal(data, msg)
}
