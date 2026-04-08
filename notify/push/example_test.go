package push_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/notify/push"
)

func ExampleNewFCMProvider() {
	provider := push.NewFCMProvider(push.FCMConfig{
		ProjectID: "my-project",
	})
	fmt.Println(provider.Name())
	// Output:
	// fcm
}

func ExampleNewAPNsProvider() {
	provider := push.NewAPNsProvider(push.APNsConfig{
		TeamID: "TEAM123",
		KeyID:  "KEY456",
	})
	fmt.Println(provider.Name())
	// Output:
	// apns
}
