package oauth2_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/oauth2"
)

func ExampleToken_IsExpired() {
	token := &oauth2.Token{
		AccessToken: "abc123",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	fmt.Println(token.IsExpired())

	expired := &oauth2.Token{
		AccessToken: "xyz789",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}
	fmt.Println(expired.IsExpired())
	// Output:
	// false
	// true
}
