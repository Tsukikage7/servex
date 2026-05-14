// Package proto 提供 Protobuf JSON 编解码器实现.
// 对 proto.Message 使用 protojson 序列化，其他类型回退到标准 JSON.
package proto

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"github.com/Tsukikage7/servex/v2/encoding"
	"github.com/Tsukikage7/servex/v2/encoding/pbjson"
)

func init() { encoding.RegisterCodec(codec{}) }

type codec struct{}

func (codec) Marshal(v any) ([]byte, error) {
	if msg, ok := v.(proto.Message); ok {
		return pbjson.Marshal(msg)
	}
	// 非 proto.Message 回退到标准 JSON
	return json.Marshal(v)
}

func (codec) Unmarshal(data []byte, v any) error {
	if msg, ok := v.(proto.Message); ok {
		return pbjson.Unmarshal(data, msg)
	}
	// 非 proto.Message 回退到标准 JSON
	return json.Unmarshal(data, v)
}

func (codec) Name() string { return "proto" }
