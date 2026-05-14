// Package push registers the push notify factory.
package push

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/notify"
	"github.com/Tsukikage7/servex/v2/notify/factory"
	"github.com/Tsukikage7/servex/v2/notify/push"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterSender("push", func(cfg *factory.Config, log logger.Logger) (notify.Sender, error) {
		pushConfig := cfg.Push
		var provider push.Provider
		switch pushConfig.Provider {
		case "fcm":
			if pushConfig.FCM == nil {
				pushConfig.FCM = &factory.FCMPushConfig{}
			}
			provider = push.NewFCMProvider(push.FCMConfig{
				ProjectID:       pushConfig.FCM.ProjectID,
				CredentialsJSON: []byte(pushConfig.FCM.CredentialsJSON),
			})
		case "apns":
			if pushConfig.APNs == nil {
				pushConfig.APNs = &factory.APNsPushConfig{}
			}
			provider = push.NewAPNsProvider(push.APNsConfig{
				BundleID:   pushConfig.APNs.BundleID,
				TeamID:     pushConfig.APNs.TeamID,
				KeyID:      pushConfig.APNs.KeyID,
				KeyFile:    pushConfig.APNs.KeyFile,
				Production: pushConfig.APNs.Production,
			})
		default:
			return nil, fmt.Errorf("不支持的 push provider %q", pushConfig.Provider)
		}

		var opts []push.Option
		if log != nil {
			opts = append(opts, push.WithLogger(log))
		}
		return push.NewSender(provider, opts...)
	})
}
