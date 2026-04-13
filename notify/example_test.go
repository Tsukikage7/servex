package notify_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/notify"
)

func ExampleMessage() {
	msg := &notify.Message{
		Channel: notify.ChannelEmail,
		To:      []string{"user@example.com"},
		Subject: "Welcome",
		Body:    "Hello, welcome to our platform!",
	}
	fmt.Println(msg.Channel)
	fmt.Println(msg.Channel.Valid())
	fmt.Println(msg.To[0])
	// Output:
	// email
	// true
	// user@example.com
}

func ExampleValidateMessage() {
	err := notify.ValidateMessage(&notify.Message{
		Channel: notify.ChannelSMS,
		To:      []string{"+1234567890"},
	})
	fmt.Println(err)
	// Output:
	// <nil>
}
