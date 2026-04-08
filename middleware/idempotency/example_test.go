package idempotency_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/middleware/idempotency"
)

func ExampleResult_Encode() {
	result := &idempotency.Result{
		StatusCode: 200,
		Body:       []byte(`{"ok":true}`),
		CreatedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := result.Encode()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("encoded:", len(data) > 0)

	// Decode it back.
	decoded, err := idempotency.DecodeResult(data)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("status:", decoded.StatusCode)
	fmt.Println("body:", string(decoded.Body))
	// Output:
	// encoded: true
	// status: 200
	// body: {"ok":true}
}
