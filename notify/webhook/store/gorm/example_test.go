package gorm_test

import (
	"fmt"

	whstoregorm "github.com/Tsukikage7/servex/v2/notify/webhook/store/gorm"
)

func ExampleWithTableName() {
	opt := whstoregorm.WithTableName("custom_subscriptions")
	fmt.Println(opt != nil)
	// Output:
	// true
}
