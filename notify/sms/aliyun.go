package sms

import (
	"context"

	"github.com/Tsukikage7/servex/notify"
)

// AliyunConfig 阿里云短信服务配置.
type AliyunConfig struct {
	AccessKeyID     string `json:"access_key_id"     yaml:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret" yaml:"access_key_secret"`
	SignName        string `json:"sign_name"         yaml:"sign_name"`
	Endpoint        string `json:"endpoint"          yaml:"endpoint"`
}

// AliyunProvider 阿里云短信服务提供者.
type AliyunProvider struct{ config AliyunConfig }

// NewAliyunProvider 创建阿里云短信提供者.
func NewAliyunProvider(cfg AliyunConfig) *AliyunProvider { return &AliyunProvider{config: cfg} }

// Name 返回提供者名称.
func (p *AliyunProvider) Name() string { return "aliyun" }

// Send 桩实现。TODO: 接入阿里云 SMS SDK.
func (p *AliyunProvider) Send(_ context.Context, _ *SendRequest) (string, error) {
	return "", notify.ErrNotImplemented
}
