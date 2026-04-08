package deviceinfo_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/httpx/deviceinfo"
)

func ExampleParser_Parse() {
	parser := deviceinfo.New()

	info := parser.Parse(deviceinfo.Headers{
		SecCHUA:         `"Chromium";v="120", "Google Chrome";v="120", "Not-A.Brand";v="24"`,
		SecCHUAMobile:   "?0",
		SecCHUAPlatform: `"Windows"`,
	})

	fmt.Println(info.IsMobile)
	fmt.Println(info.Platform)
	fmt.Println(info.Browser)
	fmt.Println(info.Source)
	// Output:
	// false
	// Windows
	// Google Chrome
	// client-hints
}
