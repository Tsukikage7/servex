package google_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/oauth2/google"
)

func ExampleNewProvider() {
	p := google.NewProvider(
		google.WithClientID("client-id"),
		google.WithClientSecret("client-secret"),
		google.WithRedirectURL("https://example.com/callback"),
		google.WithScopes("openid", "email"),
	)
	fmt.Println(p != nil)
	// Output:
	// true
}
