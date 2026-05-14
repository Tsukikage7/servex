// Package email registers the email notify factory.
package email

import (
	"github.com/Tsukikage7/servex/v2/notify"
	"github.com/Tsukikage7/servex/v2/notify/email"
	"github.com/Tsukikage7/servex/v2/notify/factory"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterSender("email", func(cfg *factory.Config, _ logger.Logger) (notify.Sender, error) {
		eo := []email.Option{
			email.WithSMTP(cfg.Email.Host, cfg.Email.Port),
			email.WithFrom(cfg.Email.From, cfg.Email.Name),
		}
		if cfg.Email.Username != "" {
			eo = append(eo, email.WithAuth(cfg.Email.Username, cfg.Email.Password))
		}
		if cfg.Email.TLS {
			eo = append(eo, email.WithTLS(true))
		}
		return email.NewSender(eo...)
	})
}
