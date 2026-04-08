package sms_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/notify/sms"
)

func ExampleNewAliyunProvider() {
	provider := sms.NewAliyunProvider(sms.AliyunConfig{
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
	})
	fmt.Println(provider.Name())
	// Output:
	// aliyun
}

func ExampleSendRequest() {
	req := sms.SendRequest{
		Phone:        "+1234567890",
		TemplateCode: "SMS_001",
	}
	fmt.Println(req.Phone)
	// Output:
	// +1234567890
}
