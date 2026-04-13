package xml_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/encoding"
	_ "github.com/Tsukikage7/servex/v2/encoding/xml"
)

func ExampleCodec_marshal() {
	codec := encoding.GetCodec("xml")

	type Item struct {
		Name  string `xml:"name"`
		Price int    `xml:"price"`
	}

	data, _ := codec.Marshal(Item{Name: "Book", Price: 42})
	fmt.Println(string(data))
	// Output: <Item><name>Book</name><price>42</price></Item>
}
