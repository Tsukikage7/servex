// Package webhook registers the webhook notify factory.
package webhook

import (
	"time"

	"github.com/Tsukikage7/servex/v2/notify"
	"github.com/Tsukikage7/servex/v2/notify/factory"
	"github.com/Tsukikage7/servex/v2/notify/nwebhook"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterSender("webhook", func(cfg *factory.Config, _ logger.Logger) (notify.Sender, error) {
		wo := []nwebhook.Option{}
		if cfg.Webhook.Timeout > 0 {
			wo = append(wo, nwebhook.WithTimeout(time.Duration(cfg.Webhook.Timeout)*time.Second))
		}
		if cfg.Webhook.Retry > 0 {
			wo = append(wo, nwebhook.WithRetry(cfg.Webhook.Retry))
		}
		return nwebhook.NewSender(wo...)
	})
}
