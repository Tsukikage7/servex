package database_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/database"
)

func ExampleWithTableName() {
	opt := database.WithTableName("custom_jobs")
	fmt.Println(opt != nil)
	// Output:
	// true
}
