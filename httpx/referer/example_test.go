package referer_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/httpx/referer"
)

func ExampleParse() {
	ref := referer.Parse("https://www.google.com/search?q=servex")
	fmt.Println(ref.Type)
	fmt.Println(ref.Source)
	fmt.Println(ref.Domain)
	// Output:
	// search
	// google
	// www.google.com
}
