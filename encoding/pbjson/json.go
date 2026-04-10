// Package pbjson 提供 protobuf JSON 序列化工具.
package pbjson

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// 全局选项为包级常量，不应在运行时修改.
var (
	// MarshalOptions 序列化选项，输出零值字段.
	MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true, // 输出零值字段
	}

	// UnmarshalOptions 反序列化选项.
	UnmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true, // 忽略未知字段
	}
)

// Marshal 将 proto 消息序列化为 JSON，包含零值字段.
func Marshal(m proto.Message) ([]byte, error) {
	return MarshalOptions.Marshal(m)
}

// Unmarshal 将 JSON 反序列化为 proto 消息.
func Unmarshal(data []byte, m proto.Message) error {
	return UnmarshalOptions.Unmarshal(data, m)
}

// MarshalString 将 proto 消息序列化为 JSON 字符串.
func MarshalString(m proto.Message) (string, error) {
	data, err := Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
