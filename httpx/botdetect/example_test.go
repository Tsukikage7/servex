package botdetect_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/httpx/botdetect"
)

func ExampleDetector_Detect() {
	detector := botdetect.New()

	result := detector.Detect("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	fmt.Println(result.IsBot)
	fmt.Println(result.Category)
	fmt.Println(result.Name)
	// Output:
	// true
	// search
	// Googlebot
}
