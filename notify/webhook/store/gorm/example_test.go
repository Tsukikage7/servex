package gorm_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/notify/webhook/store/gorm"
)

func ExampleWithTableName() {
	opt := gorm.WithTableName("custom_subscriptions")
	fmt.Println(opt != nil)
	// Output:
	// true
}
