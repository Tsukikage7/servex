package locale_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/httpx/locale"
)

func ExampleParse() {
	loc := locale.Parse("zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	fmt.Println(loc.Language())
	fmt.Println(loc.Region())
	fmt.Println(len(loc.Preferred))
	// Output:
	// zh
	// CN
	// 4
}
