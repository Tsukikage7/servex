package encoding_test

import (
	stdjson "encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/encoding"
)

// jsonCodec 用于 Example 的 JSON 编解码器.
type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return stdjson.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return stdjson.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return "json" }

// xmlCodec 用于 Example 的 XML 编解码器桩.
type xmlCodec struct{}

func (xmlCodec) Marshal(v any) ([]byte, error)      { return nil, nil }
func (xmlCodec) Unmarshal(data []byte, v any) error { return nil }
func (xmlCodec) Name() string                       { return "xml" }

// registerExampleCodecs 在每个 Example 中显式注册，避免依赖 init() 副作用.
func registerExampleCodecs() {
	encoding.RegisterCodec(jsonCodec{})
	encoding.RegisterCodec(xmlCodec{})
}

func ExampleRegisterCodec() {
	// 注册自定义编解码器
	encoding.RegisterCodec(jsonCodec{})

	// 获取已注册的编解码器
	codec := encoding.GetCodec("json")
	fmt.Println(codec.Name())
	// Output: json
}

func ExampleCodecForRequest_json() {
	registerExampleCodecs()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")

	codec := encoding.CodecForRequest(r, "Accept")
	fmt.Println(codec.Name())
	// Output: json
}

func ExampleCodecForRequest_xml() {
	registerExampleCodecs()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/xml")

	codec := encoding.CodecForRequest(r, "Accept")
	fmt.Println(codec.Name())
	// Output: xml
}

func ExampleCodecForRequest_fallback() {
	registerExampleCodecs()

	// Accept 头为空时回退到 JSON
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	codec := encoding.CodecForRequest(r, "Accept")
	fmt.Println(codec.Name())
	// Output: json
}

func ExampleGetCodec() {
	encoding.RegisterCodec(jsonCodec{})

	data := map[string]string{"hello": "world"}
	codec := encoding.GetCodec("json")

	bytes, _ := codec.Marshal(data)
	fmt.Println(string(bytes))
	// Output: {"hello":"world"}
}
