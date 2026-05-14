// Package json 提供 JSON 编解码器实现.
package json

import (
	"encoding/json"

	"github.com/Tsukikage7/servex/v2/encoding"
)

func init() { encoding.RegisterCodec(codec{}) }

type codec struct{}

func (codec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (codec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (codec) Name() string                       { return "json" }
