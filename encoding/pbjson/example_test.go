package pbjson_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/encoding/pbjson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ExampleMarshal() {
	msg := wrapperspb.String("hello")
	data, err := pbjson.Marshal(msg)
	fmt.Println(err)
	fmt.Println(string(data))
	// Output:
	// <nil>
	// "hello"
}
