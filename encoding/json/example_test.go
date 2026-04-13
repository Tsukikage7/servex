package json_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/encoding"
	_ "github.com/Tsukikage7/servex/v2/encoding/json"
)

func ExampleCodec_marshal() {
	codec := encoding.GetCodec("json")

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data, _ := codec.Marshal(User{Name: "Alice", Age: 30})
	fmt.Println(string(data))
	// Output: {"name":"Alice","age":30}
}

func ExampleCodec_unmarshal() {
	codec := encoding.GetCodec("json")

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var user User
	_ = codec.Unmarshal([]byte(`{"name":"Bob","age":25}`), &user)
	fmt.Println(user.Name, user.Age)
	// Output: Bob 25
}
