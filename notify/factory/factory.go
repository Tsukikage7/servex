// Package factory 提供 Config 驱动的 notify.Dispatcher 工厂.
package factory

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/errors"
	"github.com/Tsukikage7/servex/v2/notify"
	"github.com/Tsukikage7/servex/v2/notify/email"
	"github.com/Tsukikage7/servex/v2/notify/nwebhook"
	"github.com/Tsukikage7/servex/v2/notify/push"
	"github.com/Tsukikage7/servex/v2/notify/sms"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Config 聚合所有通知渠道的配置.
type Config struct {
	DefaultChannel string         `json:"default_channel" yaml:"default_channel"`
	TemplateDir    string         `json:"template_dir"    yaml:"template_dir"`
	Email          *EmailConfig   `json:"email"           yaml:"email"`
	SMS            *SMSConfig     `json:"sms"             yaml:"sms"`
	Webhook        *WebhookConfig `json:"webhook"         yaml:"webhook"`
	Push           *PushConfig    `json:"push"            yaml:"push"`
}

// EmailConfig 邮件发送配置.
type EmailConfig struct {
	Host, Username, Password, From, Name string
	Port                                 int
	TLS                                  bool
}

// SMSConfig 短信发送配置.
type SMSConfig struct {
	Provider string
	SignName string
	Aliyun   *AliyunSMSConfig
	Tencent  *TencentSMSConfig
}

// AliyunSMSConfig 阿里云短信配置.
type AliyunSMSConfig struct{ AccessKeyID, AccessKeySecret, Endpoint string }

// TencentSMSConfig 腾讯云短信配置.
type TencentSMSConfig struct{ SecretID, SecretKey, AppID, Endpoint string }

// WebhookConfig Webhook 发送配置.
type WebhookConfig struct {
	Timeout int
	Retry   int
}

// PushConfig 推送发送配置.
type PushConfig struct {
	Provider string
	FCM      *FCMPushConfig
	APNs     *APNsPushConfig
}

// FCMPushConfig Firebase Cloud Messaging 配置.
type FCMPushConfig struct {
	ProjectID       string
	CredentialsJSON string
}

// APNsPushConfig Apple Push Notification service 配置.
type APNsPushConfig struct {
	BundleID, TeamID, KeyID, KeyFile string
	Production                       bool
}

var (
	errNilConfig           = errors.NewWithKind(70051, "notify.factory.nil_config", "config 不能为空", errors.KindInvalidArgument)
	errEmailSenderFailed   = errors.NewWithKind(70052, "notify.factory.email_sender_failed", "创建 email sender 失败", errors.KindInternal)
	errWebhookSenderFailed = errors.NewWithKind(70053, "notify.factory.webhook_sender_failed", "创建 webhook sender 失败", errors.KindInternal)
	errUnsupportedSMS      = errors.NewWithKind(70054, "notify.factory.unsupported_sms_provider", "不支持的 SMS provider", errors.KindInvalidArgument)
	errUnsupportedPush     = errors.NewWithKind(70055, "notify.factory.unsupported_push_provider", "不支持的 push provider", errors.KindInvalidArgument)
)

// NewDispatcher 根据 Config 创建并配置好 *notify.Dispatcher.
func NewDispatcher(cfg *Config, log logger.Logger) (*notify.Dispatcher, error) {
	if cfg == nil {
		return nil, errNilConfig
	}

	var opts []notify.Option
	if log != nil {
		opts = append(opts, notify.WithLogger(log))
	}
	if cfg.DefaultChannel != "" {
		opts = append(opts, notify.WithDefaultChannel(notify.Channel(cfg.DefaultChannel)))
	}
	if cfg.TemplateDir != "" {
		opts = append(opts, notify.WithTemplateEngine(
			notify.NewTemplateEngine(notify.WithTemplateDir(cfg.TemplateDir)),
		))
	}

	d := notify.NewDispatcher(opts...)

	if cfg.Email != nil {
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
		s, err := email.NewSender(eo...)
		if err != nil {
			return nil, errEmailSenderFailed.WithCause(err)
		}
		d.Register(s)
	}

	if cfg.SMS != nil {
		s, err := buildSMS(cfg.SMS, log)
		if err != nil {
			return nil, err
		}
		d.Register(s)
	}

	if cfg.Webhook != nil {
		wo := []nwebhook.Option{}
		if cfg.Webhook.Timeout > 0 {
			wo = append(wo, nwebhook.WithTimeout(time.Duration(cfg.Webhook.Timeout)*time.Second))
		}
		if cfg.Webhook.Retry > 0 {
			wo = append(wo, nwebhook.WithRetry(cfg.Webhook.Retry))
		}
		s, err := nwebhook.NewSender(wo...)
		if err != nil {
			return nil, errWebhookSenderFailed.WithCause(err)
		}
		d.Register(s)
	}

	if cfg.Push != nil {
		s, err := buildPush(cfg.Push, log)
		if err != nil {
			return nil, err
		}
		d.Register(s)
	}

	return d, nil
}

func buildSMS(cfg *SMSConfig, log logger.Logger) (notify.Sender, error) {
	var p sms.Provider
	switch cfg.Provider {
	case "aliyun":
		if cfg.Aliyun == nil {
			cfg.Aliyun = &AliyunSMSConfig{}
		}
		p = sms.NewAliyunProvider(sms.AliyunConfig{
			AccessKeyID:     cfg.Aliyun.AccessKeyID,
			AccessKeySecret: cfg.Aliyun.AccessKeySecret,
			SignName:        cfg.SignName,
			Endpoint:        cfg.Aliyun.Endpoint,
		})
	case "tencent":
		if cfg.Tencent == nil {
			cfg.Tencent = &TencentSMSConfig{}
		}
		p = sms.NewTencentProvider(sms.TencentConfig{
			SecretID:  cfg.Tencent.SecretID,
			SecretKey: cfg.Tencent.SecretKey,
			AppID:     cfg.Tencent.AppID,
			SignName:  cfg.SignName,
			Endpoint:  cfg.Tencent.Endpoint,
		})
	default:
		return nil, errUnsupportedSMS.WithMessage(fmt.Sprintf("不支持的 SMS provider %q", cfg.Provider))
	}
	sopts := []sms.Option{sms.WithSignName(cfg.SignName)}
	if log != nil {
		sopts = append(sopts, sms.WithLogger(log))
	}
	return sms.NewSender(p, sopts...)
}

func buildPush(cfg *PushConfig, log logger.Logger) (notify.Sender, error) {
	var p push.Provider
	switch cfg.Provider {
	case "fcm":
		if cfg.FCM == nil {
			cfg.FCM = &FCMPushConfig{}
		}
		p = push.NewFCMProvider(push.FCMConfig{
			ProjectID:       cfg.FCM.ProjectID,
			CredentialsJSON: []byte(cfg.FCM.CredentialsJSON),
		})
	case "apns":
		if cfg.APNs == nil {
			cfg.APNs = &APNsPushConfig{}
		}
		p = push.NewAPNsProvider(push.APNsConfig{
			BundleID:   cfg.APNs.BundleID,
			TeamID:     cfg.APNs.TeamID,
			KeyID:      cfg.APNs.KeyID,
			KeyFile:    cfg.APNs.KeyFile,
			Production: cfg.APNs.Production,
		})
	default:
		return nil, errUnsupportedPush.WithMessage(fmt.Sprintf("不支持的 push provider %q", cfg.Provider))
	}
	var popts []push.Option
	if log != nil {
		popts = append(popts, push.WithLogger(log))
	}
	return push.NewSender(p, popts...)
}
