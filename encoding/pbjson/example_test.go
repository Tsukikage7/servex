package pbjson_test

import (
	"fmt"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/Tsukikage7/servex/v2/encoding/pbjson"
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
