// Package xml 提供 XML 编解码器实现.
package xml

import (
	stdxml "encoding/xml"

	"github.com/Tsukikage7/servex/v2/encoding"
)

func init() { encoding.RegisterCodec(codec{}) }

type codec struct{}

func (codec) Marshal(v any) ([]byte, error)      { return stdxml.Marshal(v) }
func (codec) Unmarshal(data []byte, v any) error { return stdxml.Unmarshal(data, v) }
func (codec) Name() string                       { return "xml" }
