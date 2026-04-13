package sqlx_test

import (
	"encoding/json"
	"fmt"

	"github.com/Tsukikage7/servex/v2/storage/sqlx"
)

func ExampleOf() {
	n := sqlx.Of("hello")
	fmt.Println("valid:", n.Valid)
	fmt.Println("value:", n.Val)
	// Output:
	// valid: true
	// value: hello
}

func ExampleNull() {
	n := sqlx.Null[string]()
	fmt.Println("valid:", n.Valid)
	// Output:
	// valid: false
}

func ExampleNullable_ValueOr() {
	present := sqlx.Of(42)
	absent := sqlx.Null[int]()

	fmt.Println("present:", present.ValueOr(0))
	fmt.Println("absent:", absent.ValueOr(-1))
	// Output:
	// present: 42
	// absent: -1
}

func ExampleNullable_MarshalJSON() {
	present := sqlx.Of("hello")
	absent := sqlx.Null[string]()

	b1, _ := json.Marshal(present)
	b2, _ := json.Marshal(absent)

	fmt.Println("present:", string(b1))
	fmt.Println("absent:", string(b2))
	// Output:
	// present: "hello"
	// absent: null
}

func ExampleNullable_UnmarshalJSON() {
	var n sqlx.Nullable[int]
	_ = json.Unmarshal([]byte("42"), &n)
	fmt.Println("value:", n.Val, "valid:", n.Valid)

	_ = json.Unmarshal([]byte("null"), &n)
	fmt.Println("valid after null:", n.Valid)
	// Output:
	// value: 42 valid: true
	// valid after null: false
}
