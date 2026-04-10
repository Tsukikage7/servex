// notification/sms/provider_test.go
package sms

import (
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/notify"
)

func TestAliyunProvider_ImplementsInterface(t *testing.T) { var _ Provider = (*AliyunProvider)(nil) }

func TestAliyunProvider_Name(t *testing.T) {
	p := NewAliyunProvider(AliyunConfig{AccessKeyID: "ak", AccessKeySecret: "sk", SignName: "Test"})
	if p.Name() != "aliyun" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestAliyunProvider_Send_Stub(t *testing.T) {
	p := NewAliyunProvider(AliyunConfig{AccessKeyID: "ak", AccessKeySecret: "sk"})
	_, err := p.Send(t.Context(), &SendRequest{Phone: "13800138000", TemplateCode: "SMS_001"})
	if !errors.Is(err, notify.ErrNotImplemented) {
		t.Errorf("got %v, want ErrNotImplemented", err)
	}
}

func TestTencentProvider_ImplementsInterface(t *testing.T) { var _ Provider = (*TencentProvider)(nil) }

func TestTencentProvider_Name(t *testing.T) {
	p := NewTencentProvider(TencentConfig{SecretID: "sid", SecretKey: "skey", AppID: "app"})
	if p.Name() != "tencent" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestTencentProvider_Send_Stub(t *testing.T) {
	p := NewTencentProvider(TencentConfig{SecretID: "sid", SecretKey: "skey", AppID: "app"})
	_, err := p.Send(t.Context(), &SendRequest{Phone: "13800138000", TemplateCode: "T_001"})
	if !errors.Is(err, notify.ErrNotImplemented) {
		t.Errorf("got %v, want ErrNotImplemented", err)
	}
}
