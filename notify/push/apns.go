package push

import (
	"context"

	"github.com/Tsukikage7/servex/v2/notify"
)

// APNsConfig Apple Push Notification service 连接配置.
type APNsConfig struct {
	BundleID   string `json:"bundle_id"  yaml:"bundle_id"`
	TeamID     string `json:"team_id"    yaml:"team_id"`
	KeyID      string `json:"key_id"     yaml:"key_id"`
	KeyFile    string `json:"key_file"   yaml:"key_file"`
	Production bool   `json:"production" yaml:"production"`
}

// APNsProvider 基于 Apple APNs 的推送提供者.
type APNsProvider struct{ config APNsConfig }

// NewAPNsProvider 创建 APNs 推送提供者.
func NewAPNsProvider(cfg APNsConfig) *APNsProvider { return &APNsProvider{config: cfg} }

// Name 返回提供者名称.
func (p *APNsProvider) Name() string { return "apns" }

// Send 桩实现。TODO: 接入 Apple APNs HTTP/2 API.
func (p *APNsProvider) Send(_ context.Context, _ string, _ *Payload) (string, error) {
	return "", notify.ErrNotImplemented
}
