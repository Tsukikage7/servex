package pagination_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/pagination"
)

func ExampleEncodeCursor() {
	cursor := pagination.EncodeCursor(42)
	fmt.Println("cursor:", cursor)

	values, err := pagination.DecodeCursor(cursor)
	fmt.Println("error:", err)
	fmt.Printf("value: %v\n", values[0])
	// Output:
	// cursor: WzQyXQ==
	// error: <nil>
	// value: 42
}

func ExampleCursorRequest_Apply() {
	req := &pagination.CursorRequest{Limit: 0}
	req.Apply()
	fmt.Println("limit:", req.Limit)
	fmt.Println("direction:", req.Direction)

	req2 := &pagination.CursorRequest{Limit: 999}
	req2.Apply()
	fmt.Println("capped limit:", req2.Limit)
	// Output:
	// limit: 20
	// direction: forward
	// capped limit: 100
}
