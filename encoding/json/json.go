// Package json 提供 JSON 编解码器实现.
package json

import (
	stdjson "encoding/json"

	"github.com/Tsukikage7/servex/v2/encoding"
)

func init() { encoding.RegisterCodec(codec{}) }

type codec struct{}

func (codec) Marshal(v any) ([]byte, error)      { return stdjson.Marshal(v) }
func (codec) Unmarshal(data []byte, v any) error { return stdjson.Unmarshal(data, v) }
func (codec) Name() string                       { return "json" }
