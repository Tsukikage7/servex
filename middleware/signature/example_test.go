package signature_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/middleware/signature"
)

func ExampleSign() {
	body := []byte(`{"amount":100}`)
	timestamp := "1700000000"
	secret := "my-secret-key"

	sig := signature.Sign(body, timestamp, secret)
	fmt.Println("signature length:", len(sig))

	// Verify the signature.
	ok := signature.Verify(body, timestamp, sig, secret)
	fmt.Println("valid:", ok)
	// Output:
	// signature length: 64
	// valid: true
}

func ExampleVerify() {
	body := []byte(`hello`)
	timestamp := "1700000000"
	secret := "secret"

	sig := signature.Sign(body, timestamp, secret)

	// Correct secret.
	fmt.Println("correct:", signature.Verify(body, timestamp, sig, secret))
	// Wrong secret.
	fmt.Println("wrong:", signature.Verify(body, timestamp, sig, "wrong"))
	// Output:
	// correct: true
	// wrong: false
}

func ExampleDefaultConfig() {
	cfg := signature.DefaultConfig("my-secret")
	fmt.Println("header:", cfg.HeaderName)
	fmt.Println("timestamp header:", cfg.TimestampHeader)
	fmt.Println("algorithm:", cfg.Algorithm)
	// Output:
	// header: X-Signature
	// timestamp header: X-Timestamp
	// algorithm: sha256
}
