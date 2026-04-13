package crypto_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/crypto"
)

func ExampleHashPassword() {
	hash, err := crypto.HashPassword("my-secret-password")
	fmt.Println(err)
	fmt.Println(len(hash) > 0)

	err = crypto.VerifyPassword(hash, "my-secret-password")
	fmt.Println(err)
	// Output:
	// <nil>
	// true
	// <nil>
}
