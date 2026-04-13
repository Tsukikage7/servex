package proto_test

import (
	"fmt"

	_ "github.com/Tsukikage7/servex/v2/encoding/proto"
)

func Example() {
	// Importing the proto package registers the protobuf codec.
	fmt.Println("codec registered")
	// Output:
	// codec registered
}
