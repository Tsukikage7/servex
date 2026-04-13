package sms

import (
	"context"

	"github.com/Tsukikage7/servex/v2/notify"
)

// TencentConfig 腾讯云短信服务配置.
type TencentConfig struct {
	SecretID  string `json:"secret_id"  yaml:"secret_id"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	AppID     string `json:"app_id"     yaml:"app_id"`
	SignName  string `json:"sign_name"  yaml:"sign_name"`
	Endpoint  string `json:"endpoint"   yaml:"endpoint"`
}

// TencentProvider 腾讯云短信服务提供者.
type TencentProvider struct{ config TencentConfig }

// NewTencentProvider 创建腾讯云短信提供者.
func NewTencentProvider(cfg TencentConfig) *TencentProvider { return &TencentProvider{config: cfg} }

// Name 返回提供者名称.
func (p *TencentProvider) Name() string { return "tencent" }

// Send 桩实现。TODO: 接入腾讯云 SMS SDK.
func (p *TencentProvider) Send(_ context.Context, _ *SendRequest) (string, error) {
	return "", notify.ErrNotImplemented
}
