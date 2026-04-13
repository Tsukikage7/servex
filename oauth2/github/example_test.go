package github_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/oauth2/github"
)

func ExampleNewProvider() {
	p := github.NewProvider(
		github.WithClientID("client-id"),
		github.WithClientSecret("client-secret"),
		github.WithRedirectURL("https://example.com/callback"),
	)
	defer p.Close()
	fmt.Println(p != nil)
	// Output:
	// true
}
