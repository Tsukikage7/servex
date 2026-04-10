package push

import (
	"context"

	"github.com/Tsukikage7/servex/notify"
)

// FCMConfig Firebase Cloud Messaging 连接配置.
type FCMConfig struct {
	ProjectID       string `json:"project_id"       yaml:"project_id"`
	CredentialsJSON []byte `json:"credentials_json" yaml:"credentials_json"`
}

// FCMProvider 基于 Firebase Cloud Messaging 的推送提供者.
type FCMProvider struct{ config FCMConfig }

// NewFCMProvider 创建 FCM 推送提供者.
func NewFCMProvider(cfg FCMConfig) *FCMProvider { return &FCMProvider{config: cfg} }

// Name 返回提供者名称.
func (p *FCMProvider) Name() string { return "fcm" }

// Send 桩实现。TODO: 接入 Firebase Admin SDK.
func (p *FCMProvider) Send(_ context.Context, _ string, _ *Payload) (string, error) {
	return "", notify.ErrNotImplemented
}
