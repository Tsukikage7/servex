package google_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/oauth2/google"
)

func ExampleNewProvider() {
	p := google.NewProvider(
		google.WithClientID("client-id"),
		google.WithClientSecret("client-secret"),
		google.WithRedirectURL("https://example.com/callback"),
		google.WithScopes("openid", "email"),
	)
	defer p.Close()
	fmt.Println(p != nil)
	// Output:
	// true
}
