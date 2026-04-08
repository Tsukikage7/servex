package tenantgorm_test

import (
	"fmt"

	tenantgorm "github.com/Tsukikage7/servex/tenant/gorm"
)

func ExampleScope() {
	fn := tenantgorm.Scope(nil)
	fmt.Println(fn != nil)
	// Output:
	// true
}
