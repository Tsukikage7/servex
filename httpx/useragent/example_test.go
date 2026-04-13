package useragent_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/httpx/useragent"
)

func ExampleParse() {
	ua := useragent.Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	fmt.Println(ua.Browser.Name)
	fmt.Println(ua.OS.Name)
	fmt.Println(ua.Device.Type)
	// Output:
	// Chrome
	// Windows
	// Desktop
}
