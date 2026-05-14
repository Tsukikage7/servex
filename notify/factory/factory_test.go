// notification/factory/factory_test.go
package factory_test

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/notify/factory"
	_ "github.com/Tsukikage7/servex/v2/notify/factory/email"
	_ "github.com/Tsukikage7/servex/v2/notify/factory/push"
	_ "github.com/Tsukikage7/servex/v2/notify/factory/sms"
	_ "github.com/Tsukikage7/servex/v2/notify/factory/webhook"
)

func TestNewDispatcher_NilConfig(t *testing.T) {
	_, err := factory.NewDispatcher(nil, nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewDispatcher_EmptyConfig(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithEmail(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{
		DefaultChannel: "email",
		Email:          &factory.EmailConfig{Host: "smtp.example.com", Port: 587, From: "noreply@example.com", Name: "Test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithSMS_Aliyun(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{SMS: &factory.SMSConfig{
		Provider: "aliyun", SignName: "Test",
		Aliyun: &factory.AliyunSMSConfig{AccessKeyID: "ak", AccessKeySecret: "sk"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithSMS_Tencent(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{SMS: &factory.SMSConfig{
		Provider: "tencent", SignName: "Test",
		Tencent: &factory.TencentSMSConfig{SecretID: "sid", SecretKey: "skey", AppID: "app"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithSMS_UnknownProvider(t *testing.T) {
	_, err := factory.NewDispatcher(&factory.Config{SMS: &factory.SMSConfig{Provider: "unknown"}}, nil)
	if err == nil {
		t.Error("expected error for unknown SMS provider")
	}
}

func TestNewDispatcher_WithWebhook(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{Webhook: &factory.WebhookConfig{Timeout: 5, Retry: 3}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithPush_FCM(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{Push: &factory.PushConfig{Provider: "fcm", FCM: &factory.FCMPushConfig{ProjectID: "proj"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithPush_APNs(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{Push: &factory.PushConfig{Provider: "apns", APNs: &factory.APNsPushConfig{BundleID: "com.example.app"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithPush_UnknownProvider(t *testing.T) {
	_, err := factory.NewDispatcher(&factory.Config{Push: &factory.PushConfig{Provider: "unknown"}}, nil)
	if err == nil {
		t.Error("expected error for unknown push provider")
	}
}

func TestNewDispatcher_AllChannels(t *testing.T) {
	d, err := factory.NewDispatcher(&factory.Config{
		DefaultChannel: "email",
		Email:          &factory.EmailConfig{Host: "smtp.example.com", Port: 587, From: "no@example.com", Name: "T"},
		SMS:            &factory.SMSConfig{Provider: "aliyun", SignName: "T", Aliyun: &factory.AliyunSMSConfig{AccessKeyID: "ak", AccessKeySecret: "sk"}},
		Webhook:        &factory.WebhookConfig{Timeout: 10, Retry: 2},
		Push:           &factory.PushConfig{Provider: "fcm", FCM: &factory.FCMPushConfig{ProjectID: "p"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}
