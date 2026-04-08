package activity_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/httpx/activity"
)

func ExampleEventType() {
	fmt.Println(activity.EventTypeRequest)
	fmt.Println(activity.EventTypeLogin)
	fmt.Println(activity.EventTypeLogout)
	// Output:
	// request
	// login
	// logout
}
